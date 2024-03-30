package fns

import (
	"context"
	"fmt"

	"github.com/henderiw-nephio/kform/kform-plugin/kfprotov1/kfplugin1"
	"github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/render2/celrenderer"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func NewProviderFn(cfg *Config) fn.BlockInstanceRunner {
	return &provider{
		rootPackageName:   cfg.RootPackageName,
		varStore:          cfg.VarStore,
		outputStore:       cfg.OutputStore,
		recorder:          cfg.Recorder,
		providers:         cfg.Providers,
		providerInstances: cfg.ProviderInstances,
		providerConfigs:   cfg.ProviderConfigs,
	}
}

type provider struct {
	// initialized from the vertexContext
	rootPackageName string
	// dynamic injection required
	varStore          store.Storer[data.VarData]
	outputStore       store.Storer[data.BlockData]
	recorder          recorder.Recorder[diag.Diagnostic]
	providers         store.Storer[types.Provider]
	providerInstances store.Storer[plugin.Provider]
	providerConfigs   store.Storer[string]
}

func (r *provider) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Debug("run instance")

	celrenderer := celrenderer.New(r.varStore, localVars)
	n, err := celrenderer.Render(ctx, vctx.Data.Get()[0].YNode()) // copy for safety
	if err != nil {
		return fmt.Errorf("cannot render config for %s", vctx.String())
	}
	// to interact with the provider we need a json byte
	rn := yaml.NewRNode(n)
	b, err := rn.MarshalJSON()
	if err != nil {
		log.Error("cannot json marshal list", "error", err.Error())
		return err
	}
	// store the config
	if err := r.providerConfigs.Create(ctx, store.ToKey(vctx.BlockName), rn.MustString()); err != nil {
		log.Error("cannot store provider config", "error", err.Error())
		return err
	}
	log.Debug("providerConfig", "config", string(b))
	// get the provider for initialization
	p, err := r.providers.Get(ctx, store.ToKey(vctx.BlockName))
	if err != nil {
		log.Error("provider not found in inventory", "err", err)
		return fmt.Errorf("provider %s not found in inventory err: %s", vctx.BlockName, err.Error())
	}
	// initialize the provider
	provider, err := p.Initializer()
	if err != nil {
		log.Error("cannot initialize provider", "err", err)
		return err
	}
	// add the provider client to the cache - delete will happen after the run
	if err := r.providerInstances.Update(ctx, store.ToKey(vctx.BlockName), provider); err != nil {
		log.Error("cannot update provider", "err", err)
		return err
	}

	// configure the provider
	cfgResp, err := provider.Configure(ctx, &kfplugin1.Configure_Request{
		Config: b,
	})
	if err != nil {
		log.Error("failed to configure provider", "error", err.Error())
		return fmt.Errorf("provider %s not found in inventory err: %s", vctx.BlockName, err.Error())
	}
	if len(cfgResp.Diagnostics) != 0 {
		log.Error("failed to configure provider", "error", cfgResp.Diagnostics)
		return fmt.Errorf("provider %s not found in inventory err: %s", vctx.BlockName, cfgResp.Diagnostics)
	}
	log.Debug("run block instance finished...")
	return nil
}
