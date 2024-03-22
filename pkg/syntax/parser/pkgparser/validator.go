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

package pkgparser

import (
	"context"
	"sync"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// validate processes each kform block and validates the syntax
func (r *PackageParser) validate(ctx context.Context, kformDataStore store.Storer[*yaml.RNode]) {
	/*
		var wg sync.WaitGroup
		for path, kformKO := range kforms {
			path := path
			kformKO := kformKO
			wg.Add(1)
			go func(kformKO *fn.KubeObject) {
				defer wg.Done()
				ctx = context.WithValue(ctx, types.CtxKeyFileName, path)
				r.processBlock(ctx, kformKO)
			}(kformKO)

		}
		wg.Wait()
	*/
	var wg sync.WaitGroup
	kformDataStore.List(ctx, func(ctx context.Context, key store.Key, rn *yaml.RNode) {
		wg.Add(1)
		go func(rn *yaml.RNode) {
			defer wg.Done()
			ctx = context.WithValue(ctx, types.CtxKeyFileName, key.Name)
			ctx = context.WithValue(ctx, types.CtxKeyIndex, key.Namespace)
			r.processBlock(ctx, rn)
		}(rn)
	})
	wg.Wait()
}

// walkTopBlock identifies the blockType
func (r *PackageParser) processBlock(ctx context.Context, rn *yaml.RNode) {
	// validate the blockType
	// if unspecified we assume the blockType is OUPUT
	// if specified we check against the defined blockTypes
	blockType := kformv1alpha1.BlockTYPE_OUTPUT
	blockTypeStr, ok := rn.GetAnnotations()[kformv1alpha1.KformAnnotationKey_BLOCK_TYPE]
	if ok {
		blockType = kformv1alpha1.GetBlockType(blockTypeStr)
	}
	if blockType == kformv1alpha1.BlockType_UNKNOWN {
		r.recorder.Record(diag.DiagErrorf("unknown blocktype, got %s", blockTypeStr))
		return
	}
	// blockType is known -> process the specifics of each blockType
	log := log.FromContext(ctx).With("blockType", blockType.String())
	log.Debug("processBlock")
	// initialize the specific blockType implementation
	bt, err := types.InitializeBlock(ctx, blockType)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
		return
	}
	ctx = context.WithValue(ctx, types.CtxKeyBlockType, blockType)
	ctx = context.WithValue(ctx, types.CtxKeyYamlRNODE, rn)
	// These are needed in all resources we set them and validate later
	// output might need the name as the resourceID if the annotation is not present
	// resources (data, resource, list) need a resource type
	//ctx = context.WithValue(ctx, types.CtxKeyResourceID, rn.GetAnnotations()[kformv1alpha1.KformAnnotationKey_RESOURCE_ID])
	//ctx = context.WithValue(ctx, types.CtxKeyResourceType, rn.GetAnnotations()[kformv1alpha1.KformAnnotationKey_RESOURCE_TYPE])
	// update the package with the specifics of the blockType
	bt.UpdatePackage(ctx)
}
