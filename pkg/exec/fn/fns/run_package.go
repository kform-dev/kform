package fns

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
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
		provider:          cfg.Provider,
		rootPackageName:   cfg.RootPackageName,
		dataStore:         cfg.DataStore,
		recorder:          cfg.Recorder,
		providerInventory: cfg.ProviderInventory,
		providerInstances: cfg.ProviderInstances,
	}
}

type pkg struct {
	// inidctaes which dag to pick for the run (provider dag or regular dag)
	provider bool
	// initialized from the vertexContext
	rootPackageName string
	// dynamic injection required
	dataStore         *data.DataStore
	recorder          recorder.Recorder[diag.Diagnostic]
	providerInventory store.Storer[types.Provider]
	providerInstances store.Storer[plugin.Provider]
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
	log := log.FromContext(ctx).With("vertexContext", vctx.String(), "provider", r.provider)
	log.Info("run instance")
	// render the new vars input
	newDataStore := &data.DataStore{Storer: memory.NewStore[*data.BlockData]()}

	// localVars represent the dynamic input data into the package/mixin
	// copy the data in the datastore
	// 1. for KRM based input this is presented as blockData where the key of localVars is data.Blockdata
	// 2. Count/ForEach stay local in the src package to copy data accross -> TBD
	for blockName, blockData := range localVars {
		fmt.Println("package input", blockName)
		data, ok := blockData.(*data.BlockData)
		if !ok {
			return fmt.Errorf("unexpected data, expecting *data.BlockData, got: %s", reflect.TypeOf(blockData).Name())
		}
		newDataStore.Update(ctx, store.ToKey(blockName), data)
	}

	// prepare and execute the dag (provider or regular dag based on the provider flag)
	// the vCtx.DAG is either the provider DAG or a regular DAG based on input
	// provider DAG(s) dont run hierarchically, so no need to propagate
	e, err := executor.NewDAGExecutor[*types.VertexContext](ctx, vctx.DAG, &executor.Config[*types.VertexContext]{
		Name: vctx.BlockName,
		From: dag.Root,
		Handler: NewExecHandler(ctx, &Config{
			// provider should not be set, since provider dag is not hierarchical
			RootPackageName:   r.rootPackageName,
			PackageName:       vctx.BlockName,
			DataStore:         newDataStore,
			Recorder:          r.recorder,
			ProviderInstances: r.providerInstances,
			ProviderInventory: r.providerInventory,
		}),
	})
	if err != nil {
		return err
	}
	success := e.Run(ctx)
	if success {
		// copy the output to the newResultStore to the original resultStore
		newBlockData := data.NewBlockData()
		newDataStore.List(ctx, func(ctx context.Context, key store.Key, blockdata *data.BlockData) {
			parts := strings.Split(key.Name, ".")
			if parts[0] == kformv1alpha1.BlockTYPE_OUTPUT.String() {
				if value, ok := blockdata.Data[data.DummyKey]; ok {
					// The result is stored in a single entry but the output keys are stored in a map
					// for all other blockTypes we use the dummyKey but here we store the
					// last element of the blockName as the key in the result
					newBlockData.Insert(parts[1], 1, 0, value)
				}
			}
		})
		if len(newBlockData.Data) > 0 {
			r.dataStore.Update(ctx, store.ToKey(vctx.BlockName), newBlockData)
		}
	}
	return nil
}
