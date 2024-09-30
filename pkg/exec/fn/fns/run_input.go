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
		varStore:        cfg.VarStore,
		outputStore:     cfg.OutputStore,
	}
}

type input struct {
	rootPackageName string
	varStore        store.Storer[data.VarData]
	outputStore     store.Storer[data.BlockData]
}

func (r *input) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	// NOTE: No forEach or count expected
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Debug("run block instance start...")
	// Dynamic input will already be initialized when calling the package/module,
	// so we first check if the blockName exists, if not we initialize the block
	// with the default block if it exists
	if _, err := r.varStore.Get(store.ToKey(vctx.BlockName)); err != nil {
		varData, err := vctx.Data.GetVarData()
		if err != nil {
			return err
		}
		r.varStore.Create(store.ToKey(vctx.BlockName), varData)
		log.Debug("input", "value", vctx.Data)
	}
	log.Debug("run block instance finished...")
	return nil
}
