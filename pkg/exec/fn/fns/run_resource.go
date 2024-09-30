package fns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform-plugin/kfprotov1/kfplugin1"
	"github.com/kform-dev/kform-plugin/plugin"
	"github.com/kform-dev/kform-sdk-go/pkg/diag"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/render2/celrenderer"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		dryRun:            cfg.DryRun,
		destroy:           cfg.Destroy,
	}
}

type resource struct {
	kind              DagRun
	rootPackageName   string
	varStore          store.Storer[data.VarData]
	outputStore       store.Storer[data.BlockData]
	providerInstances store.Storer[plugin.Provider]
	resources         store.Storer[store.Storer[data.BlockData]]
	dryRun            bool
	destroy           bool
}

func (r *resource) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	// NOTE: forEach or count expected and its respective values will be represented in localVars
	// ForEach: each.key/value
	// Count: count.index
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Debug("run block instance start...")

	// for inventory we could get multiple resources, which are not wrapped in
	// count/forEach/ etc
	for idx, rn := range vctx.Data.Get() {
		// for inventory read we can skip mutating the input yaml
		if r.kind != DagRunInventory {
			celrenderer := celrenderer.New(r.varStore, localVars)
			n, err := celrenderer.Render(ctx, vctx.Data.Get()[0].YNode()) // copy for safety
			if err != nil {
				return fmt.Errorf("cannot render config for %s", vctx.String())
			}
			rn = yaml.NewRNode(n)
		}
		// remove the kform annotations before interacting with the cluster
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
		provider, err := r.providerInstances.Get(store.ToKey(vctx.Attributes.Provider))
		if err != nil {
			log.Error("cannot get provider", "provider", vctx.Attributes.Provider, "error", err.Error())
			return err
		}
		if provider == nil {
			log.Error("provider not initialized", "provider", vctx.Attributes.Provider)
			return fmt.Errorf("provider %s not initialized for block: %s", vctx.Attributes.Provider, vctx.BlockName)
		}
		name := strings.Split(vctx.BlockName, ".")[0]
		switch vctx.BlockType {
		case kformv1alpha1.BlockTYPE_DATA:
			log.Debug("resource data", "json req", string(b))
			b, err = r.get(ctx, provider, name, b)
			if err != nil {
				// for inventory read we can ignore read errors
				// it shows the resource does not exist, since something
				// could have deleted the resource w/o cleaning the inventory
				if r.kind == DagRunInventory {
					return nil
				}
				return err
			}
		case kformv1alpha1.BlockTYPE_RESOURCE:
			log.Debug("resource data", "json req", string(b))
			if r.destroy {
				// we already did a get before
				if err := r.delete(ctx, provider, name, b); err != nil {
					return err
				}
			} else {
				rb, err := r.get(ctx, provider, name, b)
				if err != nil {
					if !strings.Contains(err.Error(), "not found") {
						log.Error("resource error get", "error", err.Error())
						return err
					}
					log.Debug("not found -> create", "data", string(b))
					b, err = r.create(ctx, provider, name, b)
					if err != nil {
						// no need for log as this is a duplicate
						return err
					}
					log.Debug("create resp", "data", string(b))
				} else {
					log.Debug("found -> update", "data", string(b))
					b, err = r.update(ctx, provider, name, b, rb)
					if err != nil {
						// no need for log as this is a duplicate
						return err
					}
					log.Debug("update resp", "data", string(b))
				}
			}
		case kformv1alpha1.BlockTYPE_LIST:
			// TBD how do we deal with a list
			resp, err := provider.ListDataSource(ctx, &kfplugin1.ListDataSource_Request{
				Name: name,
				Obj:  b,
			})
			if err != nil {
				log.Error("cannot list resource", "error", err.Error())
				return err
			}
			if diag.Diagnostics(resp.Diagnostics).HasError() {
				log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
				return diag.Diagnostics(resp.Diagnostics).Error()
			}
			b = resp.Obj
		default:
			return fmt.Errorf("unexpected blockType, expected %v, got %s", types.ResourceBlockTypes, vctx.BlockType)
		}

		if !r.destroy {
			//var v unstructured.Unstructured
			var v map[string]any
			if err := json.Unmarshal(b, &v); err != nil {
				log.Error("cannot unmarshal resp", "error", err.Error())
				return err
			}

			log.Debug("data response", "resp", string(b))

			if err := data.UpdateVarStore(ctx, r.varStore, vctx.BlockName, v, localVars); err != nil {
				return fmt.Errorf("update vars failed failed for blockName %s, err: %s", vctx.BlockName, err.Error())
			}

			// add the resource to the new resources to capture what is configured
			// in a normal dagRun this is needed for blockType = RESOURCE
			// for an inventory dagRun this is needed for blockType = DATA
			if vctx.BlockType == kformv1alpha1.BlockTYPE_RESOURCE ||
				(r.kind == DagRunInventory && vctx.BlockType == kformv1alpha1.BlockTYPE_DATA) {

				// get the pkgStore in which we store the resources actuated per package
				pkgStore, err := r.resources.Get(store.ToKey(r.rootPackageName))
				if err != nil {
					return err
				}
				// we need to fake the count for inventory dagRun read
				if r.kind == DagRunInventory && vctx.BlockType == kformv1alpha1.BlockTYPE_DATA {
					localVars[kformv1alpha1.LoopKeyItemsTotal] = vctx.Data.Len()
					localVars[kformv1alpha1.LoopKeyItemsIndex] = idx
				}
				if err := data.UpdateBlockStoreEntry(ctx, pkgStore, vctx.BlockName, rn, localVars); err != nil {
					return err
				}
			}
		}
	}

	log.Debug("run block instance finished...")
	return nil
}

func (r *resource) get(ctx context.Context, provider plugin.Provider, name string, b []byte) ([]byte, error) {
	log := log.FromContext(ctx)
	resp, err := provider.ReadDataSource(ctx, &kfplugin1.ReadDataSource_Request{
		Name: name,
		Obj:  b,
	})
	if err != nil {
		log.Error("cannot read resource", "error", err.Error())
		return nil, err
	}
	if diag.Diagnostics(resp.Diagnostics).HasError() {
		return nil, diag.Diagnostics(resp.Diagnostics).Error()
	}
	return resp.Obj, nil
}

func (r *resource) delete(ctx context.Context, provider plugin.Provider, name string, b []byte) error {
	log := log.FromContext(ctx)
	//log.Info("delete resource")
	resp, err := provider.DeleteResource(ctx, &kfplugin1.DeleteResource_Request{
		Name:   name,
		DryRun: r.dryRun,
		Obj:    b,
	})
	if err != nil {
		log.Error("cannot delete resource", "error", err.Error())
		return err
	}
	if diag.Diagnostics(resp.Diagnostics).HasError() {
		log.Error("delete request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
		return diag.Diagnostics(resp.Diagnostics).Error()
	}
	return nil
}

func (r *resource) create(ctx context.Context, provider plugin.Provider, name string, b []byte) ([]byte, error) {
	log := log.FromContext(ctx)
	resp, err := provider.CreateResource(ctx, &kfplugin1.CreateResource_Request{
		Name:   name,
		DryRun: r.dryRun,
		Obj:    b,
	})
	if err != nil {
		log.Error("cannot create resource", "error", err.Error())
		return nil, err
	}
	if diag.Diagnostics(resp.Diagnostics).HasError() {
		log.Error("create request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
		return nil, diag.Diagnostics(resp.Diagnostics).Error()
	}
	return resp.Obj, nil
}

func (r *resource) update(ctx context.Context, provider plugin.Provider, name string, newb, oldb []byte) ([]byte, error) {
	log := log.FromContext(ctx)
	resp, err := provider.UpdateResource(ctx, &kfplugin1.UpdateResource_Request{
		Name:   name,
		DryRun: r.dryRun,
		NewObj: newb,
		OldObj: oldb,
	})
	if err != nil {
		log.Error("cannot update resource", "error", err.Error())
		return nil, err
	}
	if diag.Diagnostics(resp.Diagnostics).HasError() {
		log.Error("update request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
		return nil, diag.Diagnostics(resp.Diagnostics).Error()
	}
	return resp.Obj, nil
}

type Resources struct {
	store.Storer[store.Storer[data.BlockData]]
}

func (r Resources) GetItem(ctx context.Context, pkgName, blockName string, rn *yaml.RNode) *unstructured.Unstructured {
	log := log.FromContext(ctx)
	pkgStore, err := r.Get(store.ToKey(pkgName))
	if err != nil {
		// not a worry as this means the package did not exist
		return nil
	}
	storeRn := data.GetBlockStoreEntry(ctx, pkgStore, blockName, rn)
	if storeRn == nil {
		return nil
	}
	u, err := convertRNodeToUnstructured(rn)
	if err != nil {
		log.Error("cannot convert rn to unstructured", "err", err.Error())
		return nil
	}
	return u
}

func (r Resources) Delete(ctx context.Context, pkgName, blockName string, rn *yaml.RNode) error {
	// get the pkgStore in which we store the resources actuated per package
	pkgStore, err := r.Get(store.ToKey(pkgName))
	if err != nil {
		// not a worry as this means the package did not exist
		return nil
	}
	if err := data.DeleteBlockStoreEntry(ctx, pkgStore, blockName, rn); err != nil {
		return err
	}
	return nil
}

func convertRNodeToUnstructured(rn *yaml.RNode) (*unstructured.Unstructured, error) {
	// Convert RNode directly to Unstructured
	b, err := rn.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: v}, nil
}
