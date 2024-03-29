package runner

import (
	"context"
	"path/filepath"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn/fns"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

func (r *runner) RunProviderDAG(ctx context.Context, rootPackage *types.Package, inputVars map[string]any) error {
	log := log.FromContext(ctx)
	// initialize the recorder
	runRecorder := recorder.New[diag.Diagnostic]()
	outputStore := memory.NewStore[data.BlockData]()

	// run the provider DAG
	log.Debug("create provider runner")
	rmfn := fns.NewPackageFn(&fns.Config{
		Kind:              fns.DagRunProvider,
		RootPackageName:   rootPackage.Name,
		OutputStore:       outputStore,
		Recorder:          runRecorder,
		ProviderInstances: r.providerInstances,
		Providers:         r.providers,
		PackageResources:  memory.NewStore[store.Storer[data.BlockData]](), // dummy init
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
	log.Debug("success executing provider DAG")
	return nil
}

func (r *runner) RunKformDAG(ctx context.Context, errCh chan error, rootPackage *types.Package, inputVars map[string]any, outputStore store.Storer[data.BlockData], pkgResourcesStore store.Storer[store.Storer[data.BlockData]]) {
	log := log.FromContext(ctx)
	defer close(errCh)

	runRecorder := recorder.New[diag.Diagnostic]()

	cmdPackageFn := fns.NewPackageFn(&fns.Config{
		RootPackageName:   rootPackage.Name,
		OutputStore:       outputStore,
		Recorder:          runRecorder,
		ProviderInstances: r.providerInstances,
		Providers:         r.providers,
		PackageResources:  pkgResourcesStore,
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
