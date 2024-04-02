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
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/render2/celrenderer"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		tmpDir:            cfg.TmpDir,
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
	tmpDir            *fsys.Directory
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
		// getFileName retrieves the filename of the obj to be written in the tmp dir
		fileName, err := getFileName(rn)
		if err != nil {
			log.Error("cannot get name from RNode", "error", err.Error())
			return err
		}
		// no need for mutating the yaml node when doing an inventory read
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
		provider, err := r.providerInstances.Get(ctx, store.ToKey(vctx.Attributes.Provider))
		if err != nil {
			log.Error("cannot get provider", "provider", vctx.Attributes.Provider, "error", err.Error())
			return err
		}
		name := strings.Split(vctx.BlockName, ".")[0]
		switch vctx.BlockType {
		case kformv1alpha1.BlockTYPE_DATA:
			log.Debug("resource data", "json req", string(b))
			b, err = get(ctx, provider, name, b)
			if err != nil {
				return err
			}
		case kformv1alpha1.BlockTYPE_RESOURCE:
			log.Debug("resource data", "json req", string(b))

			rb, err := get(ctx, provider, name, b)
			if err != nil {
				if !strings.Contains(err.Error(), "not found") {
					return err
				}
				b, err = create(ctx, provider, name, b, r.dryRun)
				if err != nil {
					return err
				}
			} else {
				var u unstructured.Unstructured
				if err := json.Unmarshal(rb, &u); err != nil {
					return err
				}
				b, err = update(ctx, provider, name, b, rb, r.dryRun)
				if err != nil {
					return err
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

		if vctx.BlockType == kformv1alpha1.BlockTYPE_RESOURCE ||
			(r.kind == DagRunInventory && vctx.BlockType == kformv1alpha1.BlockTYPE_DATA) {

			// print the content to a temp directory
			if err := r.print(fileName, &unstructured.Unstructured{Object: v}); err != nil {
				return err
			}
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

func get(ctx context.Context, provider plugin.Provider, name string, b []byte) ([]byte, error) {
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
		log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
		return nil, diag.Diagnostics(resp.Diagnostics).Error()
	}
	return resp.Obj, nil
}

func create(ctx context.Context, provider plugin.Provider, name string, b []byte, dryRun bool) ([]byte, error) {
	log := log.FromContext(ctx)
	resp, err := provider.CreateResource(ctx, &kfplugin1.CreateResource_Request{
		Name:   name,
		DryRun: dryRun,
		Obj:    b,
	})
	if err != nil {
		log.Error("cannot create resource", "error", err.Error())
		return nil, err
	}
	if diag.Diagnostics(resp.Diagnostics).HasError() {
		log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
		return nil, diag.Diagnostics(resp.Diagnostics).Error()
	}
	return resp.Obj, nil
}

func update(ctx context.Context, provider plugin.Provider, name string, newb, oldb []byte, dryRun bool) ([]byte, error) {
	log := log.FromContext(ctx)
	resp, err := provider.UpdateResource(ctx, &kfplugin1.UpdateResource_Request{
		Name:   name,
		DryRun: dryRun,
		NewObj: newb,
		OldObj: oldb,
	})
	if err != nil {
		log.Error("cannot update resource", "error", err.Error())
		return nil, err
	}
	if diag.Diagnostics(resp.Diagnostics).HasError() {
		log.Error("request failed", "error", diag.Diagnostics(resp.Diagnostics).Error())
		return nil, diag.Diagnostics(resp.Diagnostics).Error()
	}
	return resp.Obj, nil
}

// Print prints the object using the printer into a new file in the directory.
func (r *resource) print(fileName string, obj runtime.Object) error {
	a, err := meta.Accessor(obj)
	if err != nil {
		// The object is not a `metav1.Object`, ignore it.
		return err
	}
	a.SetManagedFields(nil)

	f, err := r.tmpDir.NewFile(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	return (&fsys.Printer{}).Print(f, obj)
}

func getFileName(rn *yaml.RNode) (string, error) {
	gv, err := schema.ParseGroupVersion(rn.GetApiVersion())
	if err != nil {
		return "", err
	}
	group := ""
	if gv.Group != "" {
		group = fmt.Sprintf("%v.", gv.Group)
	}
	return group + fmt.Sprintf(
		"%v.%v.%v.%v",
		gv.Version,
		rn.GetKind(),
		rn.GetNamespace(),
		rn.GetName(),
	), nil
}
