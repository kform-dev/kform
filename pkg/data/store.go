package data

import (
	"context"
	"fmt"
	"reflect"

	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
)

type DataStore struct {
	store.Storer[*BlockData]
}

// Updates the results in the store; for loop vars it uses the index of the loop var to store the result
// since we store the results of a given blockName in a slice []any
func (r *DataStore) UpdateData(ctx context.Context, blockName string, data any, localVars map[string]any) error {
	fmt.Println("update data", data)
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

	// if the data already exists we can add the content to it
	blockdata, err := r.Get(ctx, store.ToKey(blockName))
	if err != nil {
		// data does not exist in the dataStore
		blockdata = NewBlockData()
	}
	// variable exists in the varCache
	blockdata.Insert(DummyKey, totalInt, indexInt, data)
	r.Update(ctx, store.ToKey(blockName), blockdata)
	return nil
}

func (r DataStore) ListKeys(ctx context.Context) []string {
	keys := []string{}
	r.List(ctx, func(ctx context.Context, key store.Key, _ *BlockData) {
		keys = append(keys, key.Name)
	})
	return keys
}
