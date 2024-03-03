package loader

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/pkgio"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"github.com/kform-dev/kform/pkg/util/cctx"
)

func KformFileLoader(ctx context.Context, path string, input bool) (*kformv1alpha1.KformFile, map[string]*fn.KubeObject, error) {
	recorder := cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, types.CtxKeyRecorder)
	if recorder == nil {
		return nil, nil, fmt.Errorf("cannot load files w/o a recorder")
	}
	fmt.Println(filepath.Dir(path))

	fsys := fsys.NewDiskFS(filepath.Dir(path))

	reader := pkgio.FileReader{
		FileName:       filepath.Base(path),
		Fsys:           fsys,            // map fsys
		MatchFilesGlob: pkgio.YAMLMatch, // match only yaml files
	}
	data, err := reader.Read(ctx)
	if err != nil {
		fmt.Println("data read err", err.Error())
		return nil, nil, err
	}
	l := &kformloader{recorder: recorder}
	return l.load(ctx, data, input)
}
