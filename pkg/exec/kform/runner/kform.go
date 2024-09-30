package runner

import (
	"context"
	"path/filepath"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform-plugin/plugin"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn/fns"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/parser"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

type KformConfig struct {
	Kind         fns.DagRun
	PkgName      string
	Path         string
	ResourceData store.Storer[[]byte]
	DryRun       bool
	TmpDir       *fsys.Directory
	Destroy      bool
}

func newKformContext(cfg *KformConfig) *kformContext {
	return &kformContext{
		cfg:             cfg,
		outputStore:     memory.NewStore[data.BlockData](nil),
		resourcesStore:  memory.NewStore[store.Storer[data.BlockData]](nil),
		providerConfigs: memory.NewStore[string](nil),
	}
}

type kformContext struct {
	cfg               *KformConfig
	providers         store.Storer[types.Provider]
	providerInstances store.Storer[plugin.Provider]
	providerConfigs   store.Storer[string]
	outputStore       store.Storer[data.BlockData]
	resourcesStore    store.Storer[store.Storer[data.BlockData]]
}

func (r *kformContext) ParseAndRun(ctx context.Context, inputVars map[string]any) error {
	log := log.FromContext(ctx)
	kformRecorder := recorder.New[diag.Diagnostic]()
	ctx = context.WithValue(ctx, types.CtxKeyRecorder, kformRecorder)

	// syntax check config -> build the dag
	log.Debug("parsing packages")
	parser, err := parser.NewKformParser(ctx, &parser.Config{
		PackageName:  r.cfg.PkgName,
		Path:         r.cfg.Path,
		ResourceData: r.cfg.ResourceData,
	})
	if err != nil {
		return err
	}

	parser.Parse(ctx)
	if kformRecorder.Get().HasError() {
		//kformRecorder.Print()
		log.Error("failed parsing packages", "error", kformRecorder.Get().Error())
		return kformRecorder.Get().Error()
	}
	//kformRecorder.Print()

	// initialize providers which hold the identities of the raw providers
	// that reference the exec/initialization to startup the binaries
	r.providers, err = parser.InitProviders(ctx)
	if err != nil {
		log.Error("failed initializing providers", "error", err)
		return err
	}
	// Based on the used provider configs return the providerInstances
	// this is an empty list which will be initialized during the run
	r.providerInstances, err = parser.GetEmptyProviderInstances(ctx)
	if err != nil {
		log.Error("failed initializing provider Instances", "error", err)
		return err
	}

	rootPackage, err := parser.GetRootPackage(ctx)
	if err != nil {
		return err
	}
	//rootPackage.DAG.Print("root")

	// run the provider DAG
	if err := r.runProviderDAG(ctx, rootPackage, inputVars); err != nil {
		return err
	}

	defer func() {
		r.providerInstances.List(func(key store.Key, provider plugin.Provider) {
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
	// run go routine
	go r.runKformDAG(ctx, errCh, rootPackage, inputVars)
	// wait for kform dag to finish
	err = <-errCh
	if err != nil {
		log.Error("exec failed", "kind", r.cfg.Kind.String(), "err", err)
		return err
	}
	return nil
}

func (r *kformContext) getOutputStore() store.Storer[data.BlockData] {
	return r.outputStore
}

func (r *kformContext) getResources() store.Storer[store.Storer[data.BlockData]] {
	return r.resourcesStore
}

func (r *kformContext) getProviders() map[string]string {
	providers := map[string]string{}
	r.providerConfigs.List(func(k store.Key, s string) {
		providers[k.Name] = s
	})
	return providers
}

func (r *kformContext) runProviderDAG(ctx context.Context, rootPackage *types.Package, inputVars map[string]any) error {
	log := log.FromContext(ctx)
	// initialize the recorder
	runRecorder := recorder.New[diag.Diagnostic]()
	outputStore := memory.NewStore[data.BlockData](nil)

	// run the provider DAG
	log.Debug("create provider runner")
	rmfn := fns.NewPackageFn(&fns.Config{
		Kind:              fns.DagRunProvider,
		RootPackageName:   rootPackage.Name,
		OutputStore:       outputStore,
		Recorder:          runRecorder,
		ProviderInstances: r.providerInstances,
		Providers:         r.providers,
		ProviderConfigs:   r.providerConfigs,
	})
	log.Debug("executing provider runner DAG")
	if err := rmfn.Run(ctx, &types.VertexContext{
		FileName:    filepath.Join(r.cfg.Path, "provider"),
		PackageName: rootPackage.Name,
		BlockType:   kformv1alpha1.BlockTYPE_PACKAGE,
		BlockName:   rootPackage.Name,
		DAG:         rootPackage.ProviderDAG, // we supply the provider DAG here
	}, inputVars); err != nil {
		log.Error("failed running provider DAG", "err", err)
		return err
	}
	if runRecorder.Get().HasError() {
		log.Debug("failed executing provider DAG")
		return runRecorder.Get().Error()
	}

	log.Debug("success executing provider DAG")
	return nil
}

func (r *kformContext) runKformDAG(ctx context.Context, errCh chan error, rootPackage *types.Package, inputVars map[string]any) {
	log := log.FromContext(ctx)
	defer close(errCh)

	runRecorder := recorder.New[diag.Diagnostic]()

	cmdPackageFn := fns.NewPackageFn(&fns.Config{
		Kind:              r.cfg.Kind,
		RootPackageName:   rootPackage.Name,
		OutputStore:       r.outputStore,
		Recorder:          runRecorder,
		ProviderInstances: r.providerInstances,
		Providers:         r.providers,
		Resources:         r.resourcesStore,
		DryRun:            r.cfg.DryRun,
		Destroy:           r.cfg.Destroy,
	})

	log.Debug("executing package")
	if err := cmdPackageFn.Run(ctx, &types.VertexContext{
		FileName:    filepath.Join(r.cfg.Path, "provider"),
		PackageName: rootPackage.Name,
		BlockType:   kformv1alpha1.BlockTYPE_PACKAGE,
		BlockName:   rootPackage.Name,
		DAG:         rootPackage.DAG,
	}, inputVars); err != nil {
		log.Error("failed executing package", "err", err)
		errCh <- err
		return
	}

	if runRecorder.Get().HasError() {
		errCh <- runRecorder.Get().Error()
		return
	}
	errCh <- nil
}
