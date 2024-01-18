package fns

import (
	"context"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

func NewRootFn(cfg *Config) fn.BlockInstanceRunner {
	return &root{
		rootPackageName: cfg.RootPackageName,
	}
}

type root struct {
	rootPackageName string
}

func (r *root) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Info("run block instance start...")
	log.Info("run block instance finished...")
	return nil
}
