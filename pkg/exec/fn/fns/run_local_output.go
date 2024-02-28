package fns

import (
	"context"
	"fmt"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/render/celrender"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

func NewLocalOrOutputFn(cfg *Config) fn.BlockInstanceRunner {
	return &localOrOutput{
		rootPackageName: cfg.RootPackageName,
		dataStore:       cfg.DataStore,
	}
}

type localOrOutput struct {
	rootPackageName string
	dataStore       *data.DataStore
}

func (r *localOrOutput) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	// NOTE: forEach or count expected and its respective values will be represented in localVars
	// ForEach: each.key/value
	// Count: count.index
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Info("run block instance start...")
	// if the BlockContext Value is defined we render the expected output
	// the syntax parser should validate this, meaning the value should always be defined
	renderer := celrender.New(r.dataStore, localVars)
	inputData, err := types.DeepCopy(vctx.Data.Data[data.DummyKey][0])
	if err != nil {
		return err
	}
	value, err := renderer.Render(ctx, inputData)
	if err != nil {
		return err
	}
	if err := r.dataStore.UpdateData(ctx, vctx.BlockName, value, localVars); err != nil {
		return fmt.Errorf("update vars failed failed for blockName %s, err: %s", vctx.BlockName, err.Error())
	}

	log.Info("run block instance finished...")
	return nil
}
