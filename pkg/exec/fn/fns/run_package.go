package fns

import (
	"context"
	"fmt"
	"reflect"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform-plugin/plugin"
	"github.com/kform-dev/kform/pkg/dag"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/executor"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

func NewPackageFn(cfg *Config) fn.BlockInstanceRunner {
	return &pkg{
		kind:              cfg.Kind,
		rootPackageName:   cfg.RootPackageName,
		outputStore:       cfg.OutputStore,
		recorder:          cfg.Recorder,
		providers:         cfg.Providers,
		providerInstances: cfg.ProviderInstances,
		providerConfigs:   cfg.ProviderConfigs,
		resources:         cfg.Resources,
		dryRun:            cfg.DryRun,
		destroy:           cfg.Destroy,
	}
}

type pkg struct {
	kind DagRun
	// initialized from the vertexContext
	rootPackageName string
	// dynamic injection required
	outputStore       store.Storer[data.BlockData]
	recorder          recorder.Recorder[diag.Diagnostic]
	providers         store.Storer[types.Provider]
	providerInstances store.Storer[plugin.Provider]
	providerConfigs   store.Storer[string]
	resources         store.Storer[store.Storer[data.BlockData]]
	dryRun            bool
	destroy           bool
}

/*
this function is called based on count/for_each/singleton

Per execution instance (single or range (count/for_each))
1. prepare dynamic input (uses the for_each/count if relevant)
	root package -> input comes from cmdline or environment variables
				-> copy to the resultStore of the child module
	mixin package -> input comes from the parent modules variable
				-> copy to the vars cache of the child module
2. execute the dag and dedicated vars context

3. if ok copy the output from the mixin package to the root package
*/

func (r *pkg) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Debug("run instance")
	// create a new outputStore and varStore and an instance of the packageResources
	newOutputStore := memory.NewStore[data.BlockData](nil)
	newVarStore := memory.NewStore[data.VarData](nil)
	newPkgResourceStore := memory.NewStore[data.BlockData](nil)
	if r.resources != nil {
		// protection such that kform runs who dont request resources will not crash
		// e.g. a provider run
		r.resources.Create(store.ToKey(r.rootPackageName), newPkgResourceStore)
	}

	// localVars represent the dynamic input data into the package/mixin
	// copy the data in the datastore
	// 1. for KRM based input this is presented as blockData where the key of localVars is data.Blockdata
	// 2. Count/ForEach stay local in the src package to copy data accross -> TBD
	for blockName, blockData := range localVars {
		data, ok := blockData.(data.VarData)
		if !ok {
			return fmt.Errorf("unexpected data, expecting *data.BlockData, got: %s", reflect.TypeOf(blockData).Name())
		}
		newVarStore.Update(store.ToKey(blockName), data)
	}

	// TODO add warning when an inputresource is specified and its corresponding dag entry does not exist

	// prepare and execute the dag (provider or regular dag based on the provider flag)
	// the vCtx.DAG is either the provider DAG or a regular DAG based on input
	// provider DAG(s) dont run hierarchically, so no need to propagate
	e, err := executor.NewDAGExecutor[*types.VertexContext](ctx, vctx.DAG, &executor.Config[*types.VertexContext]{
		Name: vctx.BlockName,
		From: dag.Root,
		Handler: NewExecHandler(ctx, &Config{
			Kind: r.kind,
			// provider should not be set, since provider dag is not hierarchical
			RootPackageName:   r.rootPackageName,
			PackageName:       vctx.BlockName,
			VarStore:          newVarStore,
			OutputStore:       newOutputStore,
			Recorder:          r.recorder,
			ProviderInstances: r.providerInstances,
			Providers:         r.providers,
			ProviderConfigs:   r.providerConfigs,
			Resources:         r.resources,
			DryRun:            r.dryRun,
			Destroy:           r.destroy,
		}),
	})
	if err != nil {
		return err
	}
	success := e.Run(ctx)
	if success {
		// copy the output from newOutputStore to outputStore
		// Every package works independently, so this ensure isolation
		newOutputStore.List(func(k store.Key, bd data.BlockData) {
			// TODO output prefix needs to be replaced with mixin.packagename.<outputvariable>
			r.outputStore.Create(k, bd)
		})
	}
	return nil
}
