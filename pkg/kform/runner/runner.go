package runner

import (
	"context"
	"fmt"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn/fns"
	"github.com/kform-dev/kform/pkg/inventory/manager"
	"github.com/kform-dev/kform/pkg/pkgio"
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
}

func NewKformRunner(cfg *Config) Runner {
	return &runner{
		cfg: cfg,
	}
}

type runner struct {
	cfg *Config
	outputSink pkgio.OutputSink
	invManager manager.Manager
}

func (r *runner) Run(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Debug("run")

	var err error
	r.invManager, err = manager.New(ctx, r.cfg.Path, r.cfg.Factory, kformv1alpha1.ActuationStrategyApply)
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

	invkformCtx := newKformContext(fns.DagRunInventory, r.cfg.PackageName, r.cfg.Path, invResources)
	if err := invkformCtx.ParseAndRun(ctx, map[string]any{}); err != nil {
		return err
	}

	existingActuatedResources := invkformCtx.getResources()
	listPackageResources(ctx, "inv", existingActuatedResources)

	inputVars, err := r.getInputVars(ctx)
	if err != nil {
		return err
	}

	r.outputSink, err = r.getOuputSink(ctx)
	if err != nil {
		return err
	}

	kformCtx := newKformContext(fns.DagRunRegular, r.cfg.PackageName, r.cfg.Path, nil)
	if err := kformCtx.ParseAndRun(ctx, inputVars); err != nil {
		return err
	}

	outputStore := kformCtx.getOutputStore()
	providers := kformCtx.getProviders(ctx)
	newActuatedResources := kformCtx.getResources()
	listPackageResources(ctx, "new", newActuatedResources)

	if err := r.invManager.Apply(ctx, providers, newActuatedResources); err != nil {
		return err
	}

	w := pkgio.KformWriter{
		Type: r.outputSink,
		Path: r.cfg.Output,
	}
	return w.Write(ctx, outputStore)
}

func listPackageResources(ctx context.Context, prefix string, pkgResourcesStore store.Storer[store.Storer[data.BlockData]]) {
	pkgResourcesStore.List(ctx, func(ctx context.Context, k store.Key, s store.Storer[data.BlockData]) {
		pkgName := k.Name
		s.List(ctx, func(ctx context.Context, k store.Key, bd data.BlockData) {
			for idx, rn := range bd.Get() {
				fmt.Println("pkgResource", prefix, pkgName, k.String(), "idx", idx, rn.GetApiVersion(), rn.GetKind(), rn.GetName(), rn.GetNamespace())
			}
		})
	})
}
