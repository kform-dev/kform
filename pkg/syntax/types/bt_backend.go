package types

import (
	"context"
	"fmt"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/util/cctx"
)

func newBackend(ctx context.Context) BlockProcessor {
	return &backend{
		blockValidator: &blockValidator{
			expectedAnnotations: map[string]bool{
				kformv1alpha1.KformAnnotationKey_RESOURCE_ID: mandatory,
				kformv1alpha1.KformAnnotationKey_SOURCE:      mandatory,
				kformv1alpha1.KformAnnotationKey_VERSION:     mandatory,
			},
			recorder: cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, CtxKeyRecorder),
		},
	}
}

type backend struct {
	*blockValidator
}

func (r *backend) UpdatePackage(ctx context.Context) {
	// get the block name
	blockType := cctx.GetContextValue[kformv1alpha1.BlockType](ctx, CtxKeyBlockType)
	ko := cctx.GetContextValue[*fn.KubeObject](ctx, CtxKeyKubeObject)
	name := cctx.GetContextValue[string](ctx, CtxKeyResourceID)
	if name == "" {
		name = ko.GetName()
	}
	blockName := fmt.Sprintf("%s.%s", blockType.String(), name)

	// this records the errors
	r.validateAnnotations(ctx, ko)

	// update pkg -> get package from context
	pkg := cctx.GetContextValue[*Package](ctx, CtxKeyPackage)
	if pkg == nil {
		r.recorder.Record(diag.DiagFromErrWithContext(Context{ctx}.String(), fmt.Errorf("cannot add block without package")))
		return
	}

	// NOTE only expecting 1 backend
	if pkg.Backend != nil {

		block, err := NewBlock(ctx, blockType, blockName, ko)
		if err != nil {
			r.recorder.Record(diag.DiagFromErr(err))
			return
		}
		// checks for duplicate resources
		if err := pkg.Blocks.Create(ctx, store.ToKey(blockName), block); err != nil {
			r.recorder.Record(diag.DiagFromErrWithContext(
				Context{ctx}.String(),
				fmt.Errorf("duplicate resource with fileName: %s, name: %s, type: %s",
					block.GetFileName(),
					block.GetBlockName(),
					block.GetBlockType(),
				)))
		}
		pkg.Backend = block
	}
}
