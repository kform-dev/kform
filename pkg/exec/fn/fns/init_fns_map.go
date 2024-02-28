package fns

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

type Initializer func(*Config) fn.BlockInstanceRunner

type Map interface {
	fn.BlockInstanceRunner
}

type Config struct {
	Provider    bool // inidcates if this is a provider or a regular dag run
	PackageName string
	BlockName   string

	RootPackageName string
	DataStore       *data.DataStore
	Recorder        recorder.Recorder[diag.Diagnostic]
	// used for the provider DAG run + resources run to find the provider client
	ProviderInstances store.Storer[plugin.Provider]
	// hold the raw provider reference to the provider
	// used for the provider DAG run only
	Providers store.Storer[types.Provider]
}

func NewMap(ctx context.Context, cfg *Config) Map {
	if cfg == nil {
		cfg = &Config{}
	}
	return &fnMap{
		cfg: *cfg,
		fns: map[kformv1alpha1.BlockType]Initializer{
			kformv1alpha1.BlockTYPE_PACKAGE:  NewPackageFn,
			kformv1alpha1.BlockTYPE_INPUT:    NewInputFn,
			kformv1alpha1.BlockTYPE_OUTPUT:   NewLocalOrOutputFn,
			kformv1alpha1.BlockTYPE_LOCAL:    NewLocalOrOutputFn,
			kformv1alpha1.BlockTYPE_RESOURCE: NewResourceFn, // we handle this in the same fn
			kformv1alpha1.BlockTYPE_DATA:     NewResourceFn, // we handle this in the same fn
			kformv1alpha1.BlockTYPE_LIST:     NewResourceFn, // we handle this in the same fn
			kformv1alpha1.BlockTYPE_ROOT:     NewRootFn,
			kformv1alpha1.BlockTYPE_PROVIDER: NewProviderFn,
		},
	}
}

type fnMap struct {
	cfg Config
	m   sync.RWMutex
	fns map[kformv1alpha1.BlockType]Initializer
}

func (r *fnMap) getInitializedBlockTypes() []string {
	// No RLock needed since this is called only from Run
	rfns := make([]string, 0, len(r.fns))
	for blockType := range r.fns {
		rfns = append(rfns, blockType.String())
	}
	sort.Strings(rfns)
	return rfns
}

func (r *fnMap) init(blockType kformv1alpha1.BlockType) (fn.BlockInstanceRunner, error) {
	// No RLock needed since this is called only from Run
	initFn, ok := r.fns[blockType]
	if !ok {
		return nil, fmt.Errorf("blockType not initialized, got %s, initialized blocktypes: %v", blockType, r.getInitializedBlockTypes())
	}
	return initFn(&r.cfg), nil

}

func (r *fnMap) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	r.m.RLock()
	defer r.m.RUnlock()
	fn, err := r.init(vctx.BlockType)
	if err != nil {
		return err
	}
	return fn.Run(ctx, vctx, localVars)
}
