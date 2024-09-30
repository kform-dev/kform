package diff

import (
	"context"
	"errors"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/fsys"
	"k8s.io/apimachinery/pkg/runtime"
)

type Differ struct {
	From *DiffContext
	To   *DiffContext
}

func NewDiffer(from, to store.Storer[store.Storer[data.BlockData]]) (*Differ, error) {
	var err error
	differ := &Differ{}
	differ.From, err = NewDiffContext("FROM", from)
	if err != nil {
		return nil, err
	}
	differ.To, err = NewDiffContext("TO", to)
	if err != nil {
		return nil, err
	}
	return differ, nil
}

func (r *Differ) GetResourceToPrune() store.Storer[store.Storer[data.BlockData]] {
	return r.From.Store
}

func (r *Differ) FromPath() string {
	return r.From.Path()
}

func (r *Differ) ToPath() string {
	return r.To.Path()
}

// Run first prints the files in the respective directories and applies masking if needed for sensitive data
// On top we prune the from with the netries that are in common
// Afterwards the
func (r *Differ) Run(ctx context.Context) error {
	log := log.FromContext(ctx)
	// we walk over the to store
	var errm error
	if r.To.Store != nil {
		r.To.Store.List(func(pkgKey store.Key, pkgStore store.Storer[data.BlockData]) {
			pkgStore.List(func(blockKey store.Key, bd data.BlockData) {
				for _, toRn := range bd.Get() {
					fileName, err := fsys.GetFileName(toRn)
					if err != nil {
						log.Error("cannot get name from RNode", "error", err.Error())
						errm = errors.Join(errm, err)
						continue
					}
					to, err := convertRNodeToUnstructured(toRn)
					if err != nil {
						errm = errors.Join(errm, err)
						log.Error("cannot convert rn to unstructured", "error", err)
						continue
					}
					from := r.From.GetStoreItem(ctx, pkgKey.Name, blockKey.Name, toRn)

					if gvk := to.GetObjectKind().GroupVersionKind(); gvk.Version == "v1" && gvk.Kind == "Secret" {
						m, err := NewMasker(from, to)
						if err != nil {
							errm = errors.Join(errm, err)
							log.Error("cannot convert rn to unstructured", "error", err)
							continue
						}
						from, to = m.From(), m.To()
					}

					// print the file to the respective directories; if nil no worries
					if err := r.To.Print(fileName, to); err != nil {
						errm = errors.Join(errm, err)
						log.Error("cannot print to", "fileName", fileName, "error", err)
						continue
					}
					if err := r.From.Print(fileName, from); err != nil {
						errm = errors.Join(errm, err)
						log.Error("cannot print to", "fileName", fileName, "error", err)
						continue
					}

					if from != nil {
						// delete the item from the origin, such that we dont prune the item from the cluster
						if err := r.From.DeleteStoreItem(ctx, pkgKey.Name, blockKey.Name, toRn); err != nil {
							errm = errors.Join(errm, err)
							log.Error("cannot delete block item from store", "error", err)
							continue
						}
					}
				}
			})
		})
	}
	if r.From.Store != nil {
		// update the remainder of the from
		r.From.Store.List(func(pkgKey store.Key, pkgStore store.Storer[data.BlockData]) {
			pkgStore.List(func(blockKey store.Key, bd data.BlockData) {
				for _, fromRn := range bd.Get() {
					fileName, err := fsys.GetFileName(fromRn)
					if err != nil {
						log.Error("cannot get name from RNode", "error", err.Error())
						errm = errors.Join(errm, err)
						continue
					}
					from, err := convertRNodeToUnstructured(fromRn)
					if err != nil {
						errm = errors.Join(errm, err)
						log.Error("cannot convert rn to unstructured", "error", err)
						continue
					}
					var to runtime.Object

					if gvk := from.GetObjectKind().GroupVersionKind(); gvk.Version == "v1" && gvk.Kind == "Secret" {
						m, err := NewMasker(from, to)
						if err != nil {
							errm = errors.Join(errm, err)
							log.Error("cannot convert rn to unstructured", "error", err)
							continue
						}
						from, to = m.From(), m.To()
					}

					// print the file to the respective directories; if nil no worries
					if err := r.To.Print(fileName, to); err != nil {
						errm = errors.Join(errm, err)
						log.Error("cannot print to", "fileName", fileName, "error", err)
						continue
					}
					if err := r.From.Print(fileName, from); err != nil {
						errm = errors.Join(errm, err)
						log.Error("cannot print to", "fileName", fileName, "error", err)
						continue
					}
				}
			})
		})
	}
	return errm
}

func (r *Differ) TearDown() {
	r.From.DeleteDir() // we can ignore the error
	r.To.DeleteDir()   // we can ignore the error
}
