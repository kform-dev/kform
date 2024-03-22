package loader

/*
import (
	"context"
	"fmt"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/pkgio"
	"github.com/kform-dev/kform/pkg/pkgio/ignore"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"github.com/kform-dev/kform/pkg/util/cctx"
)

func KformDirLoader(ctx context.Context, path string, input bool) (*kformv1alpha1.KformFile, map[string]*fn.KubeObject, error) {
	recorder := cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, types.CtxKeyRecorder)
	if recorder == nil {
		return nil, nil, fmt.Errorf("cannot load files w/o a recorder")
	}
	fsys := fsys.NewDiskFS(path)

	// uses .kformignore to ignore some files
	ignoreRules := ignore.Empty(pkgio.IgnoreFileMatch[0])
	f, err := fsys.Open(pkgio.IgnoreFileMatch[0])
	if err == nil {
		// if an error is return the rules is empty, so we dont have to worry about the error
		ignoreRules, _ = ignore.Parse(f)
	}
	reader := pkgio.PkgReader{
		Path:           path,
		Fsys:           fsys,            // map fsys
		MatchFilesGlob: pkgio.YAMLMatch, // match only yaml files
		IgnoreRules:    ignoreRules,     // adhere to the ignore rules in the .kformignore file
		SkipDir:        true,            // a package is contained within a single directory, recursion is not needed
	}
	data, err := reader.Read(ctx)
	if err != nil {
		fmt.Println("data read err", err.Error())
		return nil, nil, err
	}
	l := &kformloader{recorder: recorder}
	return l.load(ctx, data, input)
}
*/
