package fns

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/henderiw-nephio/kform/kform-plugin/kfprotov1/kfplugin1"
	"github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/render/celrender"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

func NewProviderFn(cfg *Config) fn.BlockInstanceRunner {
	return &provider{
		rootPackageName:   cfg.RootPackageName,
		dataStore:         cfg.DataStore,
		recorder:          cfg.Recorder,
		providers:         cfg.Providers,
		providerInstances: cfg.ProviderInstances,
	}
}

type provider struct {
	// initialized from the vertexContext
	rootPackageName string
	// dynamic injection required
	dataStore         *data.DataStore
	recorder          recorder.Recorder[diag.Diagnostic]
	providers         store.Storer[types.Provider]
	providerInstances store.Storer[plugin.Provider]
}

func (r *provider) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Debug("run instance")

	renderer := celrender.New(r.dataStore, localVars)
	inputData, err := types.DeepCopy(vctx.Data.Data[data.DummyKey][0])
	if err != nil {
		return err
	}
	value, err := renderer.Render(ctx, inputData)
	if err != nil {
		return fmt.Errorf("cannot render config for %s", vctx.String())
	}
	log.Debug("data raw", "req", value)
	providerConfigByte, err := json.Marshal(value)
	if err != nil {
		log.Error("cannot json marshal config", "error", err.Error())
		return err
	}
	log.Info("providerConfig", "config", string(providerConfigByte))
	// initialize the provider
	p, err := r.providers.Get(ctx, store.ToKey(vctx.BlockName))
	if err != nil {
		log.Error("provider not found in inventory", "err", err)
		return fmt.Errorf("provider %s not found in inventory err: %s", vctx.BlockName, err.Error())
	}
	provider, err := p.Initializer()
	if err != nil {
		return err
	}
	// add the provider client to the cache - used to delete the provider after the run
	r.providerInstances.Update(ctx, store.ToKey(vctx.BlockName), provider)

	// configure the provider
	cfgResp, err := provider.Configure(ctx, &kfplugin1.Configure_Request{
		Config: providerConfigByte,
	})
	if err != nil {
		log.Error("failed to configure provider", "error", err.Error())
		return fmt.Errorf("provider %s not found in inventory err: %s", vctx.BlockName, err.Error())
	}
	if len(cfgResp.Diagnostics) != 0 {
		log.Error("failed to configure provider", "error", cfgResp.Diagnostics)
		return fmt.Errorf("provider %s not found in inventory err: %s", vctx.BlockName, cfgResp.Diagnostics)
	}
	log.Info("run block instance finished...")
	return nil
}
