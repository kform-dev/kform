package fns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/henderiw-nephio/kform/kform-plugin/kfprotov1/kfplugin1"
	"github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"github.com/henderiw-nephio/kform/kform-sdk-go/pkg/diag"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/render/celrender"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

func NewResourceFn(cfg *Config) fn.BlockInstanceRunner {
	return &resource{
		rootPackageName:   cfg.RootPackageName,
		dataStore:         cfg.DataStore,
		providerInstances: cfg.ProviderInstances,
	}
}

type resource struct {
	rootPackageName   string
	dataStore         *data.DataStore
	providerInstances store.Storer[plugin.Provider]
}

func (r *resource) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	// NOTE: forEach or count expected and its respective values will be represented in localVars
	// ForEach: each.key/value
	// Count: count.index
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Info("run block instance start...")

	renderer := celrender.New(r.dataStore, localVars)
	inputData, err := types.DeepCopy(vctx.Data.Data[data.DummyKey][0])
	if err != nil {
		return err
	}
	value, err := renderer.Render(ctx, inputData)
	if err != nil {
		return fmt.Errorf("cannot render config for %s", vctx.String())
	}
	log.Debug("data", "raw req", value)

	b, err := json.Marshal(value)
	if err != nil {
		log.Error("cannot json marshal list", "error", err.Error())
		return err
	}
	log.Info("data", "json req", string(b))

	// 2. run provider
	// lookup the provider in the provider instances
	// based on the blockType run either data or resource
	// add the data in the variable
	provider, err := r.providerInstances.Get(ctx, store.ToKey(vctx.Attributes.Provider))
	if err != nil {
		log.Error("cannot get provider", "provider", vctx.Attributes.Provider, "error", err.Error())
		return err
	}

	switch vctx.BlockType {
	case kformv1alpha1.BlockTYPE_DATA:
		resp, err := provider.ReadDataSource(ctx, &kfplugin1.ReadDataSource_Request{
			Name: strings.Split(vctx.BlockName, ".")[0],
			Data: b,
		})
		if err != nil {
			log.Error("cannot read resource", "error", err.Error())
			return err
		}
		if diag.Diagnostics(resp.Diagnostics).HasError() {
			log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
			return err
		}
		b = resp.Data
	case kformv1alpha1.BlockTYPE_RESOURCE:
		resp, err := provider.CreateResource(ctx, &kfplugin1.CreateResource_Request{
			Name: strings.Split(vctx.BlockName, ".")[0],
			Data: b,
		})
		if err != nil {
			log.Error("cannot read resource", "error", err.Error())
			return err
		}
		if diag.Diagnostics(resp.Diagnostics).HasError() {
			log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
			return err
		}
		b = resp.Data
	case kformv1alpha1.BlockTYPE_LIST:
		// TBD how do we deal with a list
		resp, err := provider.ListDataSource(ctx, &kfplugin1.ListDataSource_Request{
			Name: strings.Split(vctx.BlockName, ".")[0],
			Data: b,
		})
		if err != nil {
			log.Error("cannot read resource", "error", err.Error())
			return err
		}
		if diag.Diagnostics(resp.Diagnostics).HasError() {
			log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
			return err
		}
		b = resp.Data
	default:
		return fmt.Errorf("unexpected blockType, expected %v, got %s", types.ResourceBlockTypes, vctx.BlockType)
	}

	if err := json.Unmarshal(b, &value); err != nil {
		log.Error("cannot unmarshal resp", "error", err.Error())
		return err
	}
	log.Info("data response", "resp", string(b))

	if err := r.dataStore.UpdateData(ctx, vctx.BlockName, value, localVars); err != nil {
		return fmt.Errorf("update vars failed failed for blockName %s, err: %s", vctx.BlockName, err.Error())
	}

	log.Info("run block instance finished...")
	return nil
}
