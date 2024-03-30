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
	"github.com/kform-dev/kform/pkg/render2/celrenderer"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func NewResourceFn(cfg *Config) fn.BlockInstanceRunner {
	return &resource{
		kind:              cfg.Kind,
		rootPackageName:   cfg.RootPackageName,
		varStore:          cfg.VarStore,
		outputStore:       cfg.OutputStore,
		providerInstances: cfg.ProviderInstances,
		resources:         cfg.Resources,
	}
}

type resource struct {
	kind              DagRun
	rootPackageName   string
	varStore          store.Storer[data.VarData]
	outputStore       store.Storer[data.BlockData]
	providerInstances store.Storer[plugin.Provider]
	resources         store.Storer[store.Storer[data.BlockData]]
}

func (r *resource) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	// NOTE: forEach or count expected and its respective values will be represented in localVars
	// ForEach: each.key/value
	// Count: count.index
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Debug("run block instance start...")

	for idx, rn := range vctx.Data.Get() {
		// no need for mutating the yaml node when doing an inventory read
		if r.kind != DagRunInventory {
			celrenderer := celrenderer.New(r.varStore, localVars)
			n, err := celrenderer.Render(ctx, vctx.Data.Get()[0].YNode()) // copy for safety
			if err != nil {
				return fmt.Errorf("cannot render config for %s", vctx.String())
			}
			rn = yaml.NewRNode(n)
		}
		rnAnnotations := rn.GetAnnotations()
		for _, a := range kformv1alpha1.KformAnnotations {
			delete(rnAnnotations, a)
		}
		rn.SetAnnotations(rnAnnotations)

		// to interact with the provider we need a json byte
		b, err := rn.MarshalJSON()
		if err != nil {
			log.Error("cannot json marshal list", "error", err.Error())
			return err
		}

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
			log.Debug("resource data", "json req", string(b))
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
				return diag.Diagnostics(resp.Diagnostics).Error()
			}
			b = resp.Data
		case kformv1alpha1.BlockTYPE_RESOURCE:
			log.Debug("resource data", "json req", string(b))
			resp, err := provider.CreateResource(ctx, &kfplugin1.CreateResource_Request{
				Name: strings.Split(vctx.BlockName, ".")[0],
				Data: b,
			})
			if err != nil {
				log.Error("cannot apply resource", "error", err.Error())
				return err
			}
			if diag.Diagnostics(resp.Diagnostics).HasError() {
				log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
				return diag.Diagnostics(resp.Diagnostics).Error()
			}
			b = resp.Data
		case kformv1alpha1.BlockTYPE_LIST:
			// TBD how do we deal with a list
			resp, err := provider.ListDataSource(ctx, &kfplugin1.ListDataSource_Request{
				Name: strings.Split(vctx.BlockName, ".")[0],
				Data: b,
			})
			if err != nil {
				log.Error("cannot list resource", "error", err.Error())
				return err
			}
			if diag.Diagnostics(resp.Diagnostics).HasError() {
				log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
				return diag.Diagnostics(resp.Diagnostics).Error()
			}
			b = resp.Data
		default:
			return fmt.Errorf("unexpected blockType, expected %v, got %s", types.ResourceBlockTypes, vctx.BlockType)
		}

		v := map[string]any{}
		if err := json.Unmarshal(b, &v); err != nil {
			log.Error("cannot unmarshal resp", "error", err.Error())
			return err
		}
		log.Debug("data response", "resp", string(b))

		if err := data.UpdateVarStore(ctx, r.varStore, vctx.BlockName, v, localVars); err != nil {
			return fmt.Errorf("update vars failed failed for blockName %s, err: %s", vctx.BlockName, err.Error())
		}

		if vctx.BlockType == kformv1alpha1.BlockTYPE_RESOURCE ||
			(r.kind == DagRunInventory && vctx.BlockType == kformv1alpha1.BlockTYPE_DATA) {
			// get the pkgStore in which we store the resources actuated per package
			pkgStore, err := r.resources.Get(ctx, store.ToKey(r.rootPackageName))
			if err != nil {
				return err
			}

			// we need to fake the count for inventory read
			if r.kind == DagRunInventory && vctx.BlockType == kformv1alpha1.BlockTYPE_DATA {
				localVars[kformv1alpha1.LoopKeyItemsTotal] = vctx.Data.Len()
				localVars[kformv1alpha1.LoopKeyItemsIndex] = idx
			}
			if err := data.UpdateBlockStore(ctx, pkgStore, vctx.BlockName, rn, localVars); err != nil {
				return err
			}
		}
	}

	log.Debug("run block instance finished...")
	return nil
}
