/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package data

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
)

const DummyKey = "BamBoozle"

// VarData contains the data of the heap or variable stack
// For blockType package/module/mixin output we can have multiple key entries, so we store them using a key in the map
// For all other blockTypes we use a dummy key
type VarData map[string][]any

func (r VarData) Insert(key string, total, pos int, data any) error {
	if slice, ok := r[key]; !ok {
		r[key] = make([]any, total)
	} else {
		// this is a bit weird but in this app it make sense
		// since the total amount is known within a run
		if len(slice) != total {
			r[key] = make([]any, total)
		}
	}
	// Check if the position is out of bounds
	if pos < 0 || pos > len(r[key]) {
		// Should never happen
		return fmt.Errorf("pos is not within boundaries, pos %d, total %d", pos, total)
	}
	r[key][pos] = data
	return nil
}

// Updates the results in the store; for loop vars it uses the index of the loop var to store the result
// since we store the results of a given blockName in a slice []any
func UpdateVarStore(ctx context.Context, varStore store.Storer[VarData], blockName string, data any, localVars map[string]any) error {
	log := log.FromContext(ctx)
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
	log.Debug("update varStore entry", "key", store.ToKey(blockName), "totalInt", totalInt, "indexInt", indexInt, "data", data)
	varStore.UpdateWithKeyFn(ctx, store.ToKey(blockName), func(ctx context.Context, varData VarData) VarData {
		log.Debug("update varStore", "key", store.ToKey(blockName), "varData", varData, "totalInt", totalInt, "indexInt", indexInt, "data", data)
		if varData == nil {
			varData = VarData{}
		}
		if err := varData.Insert(DummyKey, totalInt, indexInt, data); err != nil {
			errors.Join(errm, err)
		}
		return varData
	})
	return errm
}
