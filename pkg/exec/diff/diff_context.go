package diff

import (
	"context"
	"encoding/json"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/fsys"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type DiffContext struct {
	dir   *fsys.Directory
	Store store.Storer[store.Storer[data.BlockData]]
}

func NewDiffContext(prefix string, s store.Storer[store.Storer[data.BlockData]]) (*DiffContext, error) {
	d, err := fsys.CreateTempDirectory(prefix)
	if err != nil {
		return nil, err
	}
	return &DiffContext{
		dir:   d,
		Store: s,
	}, nil
}

func (r *DiffContext) Print(fileName string, obj runtime.Object) error {
	return r.dir.Print(fileName, obj)
}

func (r *DiffContext) Path() string {
	return r.dir.Path
}

func (r *DiffContext) DeleteDir() error {
	return r.dir.Delete()
}

func (r *DiffContext) GetStoreItem(ctx context.Context, pkgName, blockName string, rn *yaml.RNode) runtime.Object {
	log := log.FromContext(ctx)
	pkgStore, err := r.Store.Get(ctx, store.ToKey(pkgName))
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

func (r *DiffContext) DeleteStoreItem(ctx context.Context, pkgName, blockName string, rn *yaml.RNode) error {
	// get the pkgStore in which we store the resources actuated per package
	pkgStore, err := r.Store.Get(ctx, store.ToKey(pkgName))
	if err != nil {
		// not a worry as this means the package did not exist
		return nil
	}
	if err := data.DeleteBlockStoreEntry(ctx, pkgStore, blockName, rn); err != nil {
		return err
	}
	return nil
}

func convertRNodeToUnstructured(rn *yaml.RNode) (runtime.Object, error) {
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
