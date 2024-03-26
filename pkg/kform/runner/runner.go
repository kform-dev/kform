package runner

import (
	"context"
	"fmt"

	"github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/apis/inv/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/inventory/manager"
	"github.com/kform-dev/kform/pkg/pkgio"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/parser"
	"github.com/kform-dev/kform/pkg/syntax/types"
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
	Path         string // path of the kform files
	ResourceData store.Storer[[]byte]
}

func NewKformRunner(cfg *Config) Runner {
	return &runner{
		cfg: cfg,
	}
}

type runner struct {
	cfg               *Config
	parser            *parser.KformParser
	providers         store.Storer[types.Provider]
	providerInstances store.Storer[plugin.Provider]
	outputSink        pkgio.OutputSink
	invManager        manager.Manager
}

func (r *runner) Run(ctx context.Context) error {
	log := log.FromContext(ctx)

	// initialize the inventory
	var err error
	r.invManager, err = manager.New(ctx, r.cfg.Path, r.cfg.Factory, v1alpha1.ActuationStrategyApply)
	if err != nil {
		return err
	}

	inputVars, err := r.getInputVars(ctx)
	if err != nil {
		return err
	}

	r.outputSink, err = r.getOuputSink(ctx)
	if err != nil {
		return err
	}

	kformRecorder := recorder.New[diag.Diagnostic]()
	ctx = context.WithValue(ctx, types.CtxKeyRecorder, kformRecorder)

	// syntax check config -> build the dag
	log.Debug("parsing packages")
	r.parser, err = parser.NewKformParser(ctx, &parser.Config{
		PackageName:  r.cfg.PackageName,
		Path:         r.cfg.Path,
		ResourceData: r.cfg.ResourceData,
	})
	if err != nil {
		return err
	}

	r.parser.Parse(ctx)
	if kformRecorder.Get().HasError() {
		kformRecorder.Print()
		log.Error("failed parsing packages", "error", kformRecorder.Get().Error())
		return kformRecorder.Get().Error()
	}
	kformRecorder.Print()

	// initialize providers which hold the identities of the raw providers
	// that reference the exec/initialization to startup the binaries
	r.providers, err = r.parser.InitProviders(ctx)
	if err != nil {
		log.Error("failed initializing providers", "error", err)
		return err
	}
	// Based on the used provider configs return the providerInstances
	// this is an empty list which will be initialized during the run
	r.providerInstances, err = r.parser.GetEmptyProviderInstances(ctx)
	if err != nil {
		log.Error("failed initializing provider Instances", "error", err)
		return err
	}

	rootPackage, err := r.parser.GetRootPackage(ctx)
	if err != nil {
		return err
	}

	// run the provider DAG
	if err := r.RunProviderDAG(ctx, rootPackage, inputVars); err != nil {
		return err
	}

	defer func() {
		r.providerInstances.List(ctx, func(ctx context.Context, key store.Key, provider plugin.Provider) {
			if provider != nil {
				provider.Close(ctx)
				log.Debug("closing provider", "nsn", key.Name)
			}
			log.Debug("closing provider nil", "nsn", key.Name)
		})
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 1)
	outputStore := memory.NewStore[data.BlockData]()
	pkgResourcesStore := memory.NewStore[store.Storer[data.BlockData]]()
	// run go routine
	go r.RunKformDAG(ctx, errCh, rootPackage, inputVars, outputStore, pkgResourcesStore)
	// wait for kform dag to finish
	err = <-errCh
	if err != nil {
		log.Error("exec failed", "err", err)
	}

	listPackageResources(ctx, pkgResourcesStore)

	if err := r.invManager.Apply(ctx); err != nil {
		return err
	}

	w := pkgio.KformWriter{
		Type: r.outputSink,
		Path: r.cfg.Output,
	}
	return w.Write(ctx, outputStore)
}

func listPackageResources(ctx context.Context, pkgResourcesStore store.Storer[store.Storer[data.BlockData]]) {
	pkgResourcesStore.List(ctx, func(ctx context.Context, k store.Key, s store.Storer[data.BlockData]) {
		fmt.Println("pkg", k.Name)
		s.List(ctx, func(ctx context.Context, k store.Key, bd data.BlockData) {
			for idx, rn := range bd.Get() {
				fmt.Println("resource", k.String(), "idx", idx, rn.MustString())
			}
		})
	})
}
