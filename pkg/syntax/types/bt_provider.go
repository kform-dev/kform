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

package types

import (
	"context"
	"fmt"

	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/util/cctx"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func newProvider(ctx context.Context) BlockProcessor {
	return &provider{
		blockValidator: &blockValidator{
			expectedAnnotations: map[string]bool{
				kformv1alpha1.KformAnnotationKey_BLOCK_TYPE:  mandatory,
				kformv1alpha1.KformAnnotationKey_RESOURCE_ID: mandatory,
			},
			recorder: cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, CtxKeyRecorder),
		},
	}
}

type provider struct {
	*blockValidator
}

func (r *provider) UpdatePackage(ctx context.Context) {
	blockType := cctx.GetContextValue[kformv1alpha1.BlockType](ctx, CtxKeyBlockType)
	rn := cctx.GetContextValue[*yaml.RNode](ctx, CtxKeyYamlRNODE)
	annotations := rn.GetAnnotations()
	name := annotations[kformv1alpha1.KformAnnotationKey_RESOURCE_ID]
	if name == "" {
		name = rn.GetName()
	}
	blockName := name

	// this records the errors
	r.validateAnnotations(ctx, rn)

	pkg := cctx.GetContextValue[*Package](ctx, CtxKeyPackage)
	if pkg == nil {
		r.recorder.Record(diag.DiagFromErrWithContext(Context{ctx}.String(), fmt.Errorf("cannot add block without package")))
		return
	}
	// checks if the blockName exists -> for blockType input this is allowed; for other blockTypes this is not allowed
	block, err := pkg.ProviderConfigs.Get(store.ToKey(blockName))
	if err != nil {
		block, err = NewBlock(ctx, blockType, blockName, rn)
		if err != nil {
			r.recorder.Record(diag.DiagFromErr(err))
			return
		}
		// checks for duplicate resources
		if err := pkg.ProviderConfigs.Create(store.ToKey(blockName), block); err != nil {
			r.recorder.Record(diag.DiagFromErrWithContext(
				Context{ctx}.String(),
				fmt.Errorf("duplicate resource with fileName: %s, name: %s, type: %s",
					block.GetFileName(),
					block.GetBlockName(),
					block.GetBlockType(),
				)))
		}
		return
	}
	// duplicate resources
	r.recorder.Record(diag.DiagFromErrWithContext(
		Context{ctx}.String(),
		fmt.Errorf("duplicate resource with fileName: %s, name: %s, type: %s",
			block.GetFileName(),
			block.GetBlockName(),
			block.GetBlockType(),
		)))
}
