package pkgparser

import (
	"context"
	"sync"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/henderiw/logger/log"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

// validate processes each kform block and validates the syntax
func (r *PackageParser) validate(ctx context.Context, kforms map[string]*fn.KubeObject) {
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
}

// walkTopBlock identifies the blockType
func (r *PackageParser) processBlock(ctx context.Context, kformKO *fn.KubeObject) {
	// validate the blockType
	// if unspecified we assume the blockType is OUPUT
	// if specified we check against the defined blockTypes
	blockType := kformv1alpha1.BlockTYPE_OUTPUT
	blockTypeStr, ok := kformKO.GetAnnotations()[kformv1alpha1.KformAnnotationKey_BLOCK_TYPE]
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
	ctx = context.WithValue(ctx, types.CtxKeyKubeObject, kformKO)
	// These are needed in all resources we set them and validate later
	// output might need the name as the resourceID if the annotation is not present
	// resources (data, resource, list) need a resource type
	ctx = context.WithValue(ctx, types.CtxKeyResourceID, kformKO.GetAnnotation(kformv1alpha1.KformAnnotationKey_RESOURCE_ID))
	ctx = context.WithValue(ctx, types.CtxKeyResourceType, kformKO.GetAnnotation(kformv1alpha1.KformAnnotationKey_RESOURCE_TYPE))
	// update the package with the specifics of the blockType
	bt.UpdatePackage(ctx)
}
