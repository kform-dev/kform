package runner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/diff"
	"github.com/kform-dev/kform/pkg/exec/fn/fns"
	"github.com/kform-dev/kform/pkg/inventory/manager"
	"github.com/kform-dev/kform/pkg/pkgio"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Runner interface {
	Run(ctx context.Context) error
}

type Config struct {
	Factory      util.Factory
	PackageName  string
	Input        string // used for none, file or dir
	InputData    store.Storer[[]byte]
	Output       string
	OutputData   store.Storer[[]byte]
	Path         string               // path of the kform files
	ResourceData store.Storer[[]byte] // this providers resource externally w/o having to parse from a filepath
	DryRun       bool
	Destroy      bool
	AutoApprove  bool
}

func NewKformRunner(cfg *Config) Runner {
	return &runner{
		cfg: cfg,
	}
}

type runner struct {
	cfg        *Config
	outputSink pkgio.OutputSink
	invManager manager.Manager
}

func (r *runner) Run(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Debug("run")

	var err error
	r.invManager, err = manager.New(ctx, r.cfg.Path, r.cfg.Factory, invv1alpha1.ActuationStrategyApply)
	if err != nil {
		return err
	}

	inventory, err := r.invManager.GetInventory(ctx)
	if err != nil {
		return err
	}
	invReader := pkgio.InventoryReader{}
	invResources, err := invReader.Read(ctx, inventory)
	if err != nil {
		return err
	}

	invkformCtx := newKformContext(&KformConfig{
		Kind:         fns.DagRunInventory,
		PkgName:      r.cfg.PackageName,
		Path:         r.cfg.Path,
		ResourceData: invResources,
		DryRun:       r.cfg.DryRun,
	})
	if err := invkformCtx.ParseAndRun(ctx, map[string]any{}); err != nil {
		return err
	}

	existingActuatedResources := invkformCtx.getResources()
	invProviders := invkformCtx.getProviders(ctx)
	//listPackageResources(ctx, "inv", existingActuatedResources)

	var newActuatedResources store.Storer[store.Storer[data.BlockData]]
	var outputStore store.Storer[data.BlockData]
	var kformProviders map[string]string
	// when this is not a detroy run we collect inputVars and run the kform dag
	if !r.cfg.Destroy {
		inputVars, err := r.getInputVars(ctx)
		if err != nil {
			return err
		}

		r.outputSink, err = r.getOuputSink(ctx)
		if err != nil {
			return err
		}

		kformCtx := newKformContext(&KformConfig{
			Kind:         fns.DagRunRegular,
			PkgName:      r.cfg.PackageName,
			Path:         r.cfg.Path,
			ResourceData: nil,
			DryRun:       r.cfg.DryRun,
		})
		if err := kformCtx.ParseAndRun(ctx, inputVars); err != nil {
			return err
		}
		outputStore = kformCtx.getOutputStore()
		kformProviders = kformCtx.getProviders(ctx)
		newActuatedResources = kformCtx.getResources()
	}

	//listPackageResources(ctx, "new", newActuatedResources)

	// we prepare the differ to diff both resources we collect
	// if is possible we get nil stores but the differ is able to handle this
	differ, err := diff.NewDiffer(existingActuatedResources, newActuatedResources)
	if err != nil {
		return err
	}
	defer differ.TearDown() // delete the directories when done
	if err := differ.Run(ctx); err != nil {
		return err
	}

	if r.cfg.DryRun {
		if err := diffExec(differ.FromPath(), differ.ToPath()); err != nil {
			return err
		}
	} else {
		//
		// delete the remaining resources
		listPackageResources(ctx, "inv to be deleted", differ.GetResourceToPrune())
		// get inventory resource to destroy
		invResources := getInventoryResourcesToDelete(ctx, differ.GetResourceToPrune(), invProviders)
		// invoke the kform context to destroy the resources
		invkformCtx := newKformContext(&KformConfig{
			Kind:         fns.DagRunInventory,
			PkgName:      r.cfg.PackageName,
			Path:         r.cfg.Path,
			ResourceData: invResources,
			DryRun:       r.cfg.DryRun,
			//TmpDir:       invDir, // directory where tmp inventory information is stored
			Destroy: true,
		})
		if err := invkformCtx.ParseAndRun(ctx, map[string]any{}); err != nil {
			return err
		}

		// when we detroy we delete the inventory
		if r.cfg.Destroy {
			return r.invManager.Delete(ctx)
		}
		if err := r.invManager.Apply(ctx, kformProviders, newActuatedResources); err != nil {
			return err
		}
	}

	w := pkgio.KformWriter{
		Type: r.outputSink,
		Path: r.cfg.Output,
	}
	return w.Write(ctx, outputStore)
}

func listPackageResources(ctx context.Context, prefix string, pkgResourcesStore store.Storer[store.Storer[data.BlockData]]) {
	if pkgResourcesStore != nil {
		pkgResourcesStore.List(ctx, func(ctx context.Context, k store.Key, s store.Storer[data.BlockData]) {
			pkgName := k.Name
			s.List(ctx, func(ctx context.Context, k store.Key, bd data.BlockData) {
				for idx, rn := range bd.Get() {
					fmt.Println("pkgResource", prefix, pkgName, k.String(), "idx", idx, rn.GetApiVersion(), rn.GetKind(), rn.GetName(), rn.GetNamespace(), rn.GetAnnotations())
				}
			})
		})
	}
}

func getInventoryResourcesToDelete(ctx context.Context, pkgResourcesStore store.Storer[store.Storer[data.BlockData]], providers map[string]string) store.Storer[[]byte] {
	invResources := memory.NewStore[[]byte]()
	usedProviders := sets.New[string]()
	pkgResourcesStore.List(ctx, func(ctx context.Context, k store.Key, s store.Storer[data.BlockData]) {
		s.List(ctx, func(ctx context.Context, k store.Key, bd data.BlockData) {
			for idx, rn := range bd.Get() {
				parts := strings.SplitN(k.Name, ".", 2)
				resourceType := parts[0]
				resourceID := parts[1]

				annotations := rn.GetAnnotations()
				annotations[kformv1alpha1.KformAnnotationKey_BLOCK_TYPE] = kformv1alpha1.BlockTYPE_RESOURCE.String()
				annotations[kformv1alpha1.KformAnnotationKey_RESOURCE_TYPE] = resourceType
				annotations[kformv1alpha1.KformAnnotationKey_RESOURCE_ID] = resourceID
				rn.SetAnnotations(annotations)

				usedProviders.Insert(strings.SplitN(resourceType, "_", 2)[0])

				invResources.Create(ctx, store.ToKey(fmt.Sprintf("%s_%d.yaml", k.String(), idx)), []byte(rn.MustString()))
			}
		})
	})

	for provider, config := range providers {
		if usedProviders.Has(provider) {
			invResources.Create(ctx, store.ToKey(fmt.Sprintf("%s.yaml", provider)), []byte(config))
		}
	}
	return invResources
}

// execute the diff program comparing the from path with the to path
func diffExec(from, to string) error {
	args := []string{"-u", "-N", from, to}
	cmd := exec.Command("diff", args...)
	out, err := cmd.CombinedOutput()
	exitCode := cmd.ProcessState.ExitCode()
	if err != nil {
		// existCode 0, no diff
		// exitCode 1, diff
		if exitCode > 1 {
			fmt.Printf("Command failed with exit code %d\n", exitCode)
			return err
		}
	}
	if exitCode == 1 {
		// we only print when the exit code indicates there is a diff
		fmt.Println(string(out))
	}
	return nil
}
