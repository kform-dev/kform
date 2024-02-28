package fns

import (
	"context"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

// provide and input runner, which runs per input instance
func NewInputFn(cfg *Config) fn.BlockInstanceRunner {
	return &input{
		rootPackageName: cfg.RootPackageName,
		dataStore:       cfg.DataStore,
	}
}

type input struct {
	rootPackageName string
	dataStore       *data.DataStore
}

func (r *input) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	// NOTE: No forEach or count expected
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Debug("run block instance start...")
	// Dynamic input will already be initializes, so we first check if the blockName exists
	// if not we initialize the block with the default block if it exists
	if _, err := r.dataStore.Get(ctx, store.ToKey(vctx.BlockName)); err != nil {
		r.dataStore.Create(ctx, store.ToKey(vctx.BlockName), vctx.Data)
		log.Debug("input", "value", vctx.Data)
	}
	log.Debug("run block instance finished...")
	return nil
}
