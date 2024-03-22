package loader

/*
import (
	"context"
	"fmt"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"github.com/kform-dev/kform/pkg/util/cctx"
)

func KformMemoryLoader(ctx context.Context, data store.Storer[[]byte], input bool) (*kformv1alpha1.KformFile, map[string]*fn.KubeObject, error) {
	recorder := cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, types.CtxKeyRecorder)
	if recorder == nil {
		return nil, nil, fmt.Errorf("cannot load files w/o a recorder")
	}

	l := &kformloader{recorder: recorder}
	return l.load(ctx, data, input)
}
*/
