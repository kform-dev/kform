package data

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// BlockData contains the data of a block -> can be pre-processed or post-processed
type BlockData []*yaml.RNode

// Insert inserts data in the blockdata if you know the position
func (r BlockData) Insert(total, pos int, rn *yaml.RNode) (BlockData, error) {
	if len(r) != total {
		r = make([]*yaml.RNode, total)
	}

	// Check if the position is out of bounds
	if pos < 0 || pos > len(r) {
		// Should never happen
		return r, fmt.Errorf("pos is not within boundaries, pos %d, total %d", pos, total)
	}
	r[pos] = rn

	return r, nil
}

// Insert inserts data in the blockdata if you know the position
func (r BlockData) Delete(rnode *yaml.RNode) (BlockData, error) {
	if len(r) == 0 {
		return r, nil
	}
	for i, rn := range r {
		if rn.GetNamespace() == rnode.GetNamespace() && rn.GetName() == rnode.GetName() {
			return append(r[:i], r[i+1:]...), nil
		}
	}
	return r, nil
}

func (r BlockData) Add(data *yaml.RNode) BlockData {
	return append(r, data)
}

func (r BlockData) Len() int {
	return len(r)
}

func (r BlockData) Get() []*yaml.RNode {
	d := make([]*yaml.RNode, 0, len(r))
	for _, rn := range r {
		d = append(d, rn.Copy())
	}
	return d
}

func (r BlockData) GetVarData() (VarData, error) {
	vardata := VarData{}
	vardata[DummyKey] = make([]any, 0, len(r))
	for _, rn := range r {
		d := map[string]any{}
		if err := yaml.Unmarshal([]byte(rn.MustString()), &d); err != nil {
			return nil, err
		}
		vardata[DummyKey] = append(vardata[DummyKey], d)
	}
	return vardata, nil
}

// Updates the results in the store; for loop vars it uses the index of the loop var to store the result
// since we store the results of a given blockName in a slice []any
func UpdateBlockStoreEntry(ctx context.Context, storeInstance store.Storer[BlockData], blockName string, rn *yaml.RNode, localVars map[string]any) error {
	total, ok := localVars[kformv1alpha1.LoopKeyItemsTotal]
	if !ok {
		total = 1
	}
	totalInt, ok := total.(int)
	if !ok {
		return fmt.Errorf("items.total must always be an int: got: %s", reflect.TypeOf(total))
	}

	index, ok := localVars[kformv1alpha1.LoopKeyItemsIndex]
	if !ok {
		index = 0
	}
	indexInt, ok := index.(int)
	if !ok {
		return fmt.Errorf("items.index must always be an int: got: %s", reflect.TypeOf(index))
	}
	if indexInt >= totalInt {
		return fmt.Errorf("index cannot be bigger or equal to total index: %d, totol: %d", indexInt, totalInt)
	}
	var errm error
	storeInstance.UpdateWithKeyFn(ctx, store.ToKey(blockName), func(ctx context.Context, blockData BlockData) BlockData {
		if blockData == nil {
			blockData = BlockData{}
		}
		blockData, err := blockData.Insert(totalInt, indexInt, rn)
		if err != nil {
			errors.Join(errm, err)
		}
		return blockData
	})
	return errm
}

// Delete the block
func DeleteBlockStoreEntry(ctx context.Context, storeInstance store.Storer[BlockData], blockName string, rn *yaml.RNode) error {
	var errm error
	storeInstance.UpdateWithKeyFn(ctx, store.ToKey(blockName), func(ctx context.Context, blockData BlockData) BlockData {
		if blockData == nil {
			return blockData
		}
		blockData, err := blockData.Delete(rn)
		if err != nil {
			errors.Join(errm, err)
		}
		return blockData
	})
	return errm
}
