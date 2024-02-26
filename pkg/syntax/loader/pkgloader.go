package loader

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/pkgio"
	"github.com/kform-dev/kform/pkg/pkgio/ignore"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"github.com/kform-dev/kform/pkg/util/cctx"
	koe "github.com/nephio-project/nephio/krm-functions/lib/kubeobject"
)

func GetKforms(ctx context.Context, path string, input bool) (*kformv1alpha1.KformFile, map[string]*fn.KubeObject, error) {
	recorder := cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, types.CtxKeyRecorder)
	if recorder == nil {
		return nil, nil, fmt.Errorf("cannot load files w/o a recorder")
	}
	fsys := fsys.NewDiskFS(path)

	log := log.FromContext(ctx)
	var kfile *kformv1alpha1.KformFile
	kforms := map[string]*fn.KubeObject{}

	// uses .kformignore to ignore some files
	ignoreRules := ignore.Empty(pkgio.IgnoreFileMatch[0])
	f, err := fsys.Open(pkgio.IgnoreFileMatch[0])
	if err == nil {
		// if an error is return the rules is empty, so we dont have to worry about the error
		ignoreRules, _ = ignore.Parse(f)
	}
	reader := pkgio.PkgReader{
		PathExists:     true,            // path was validated
		Fsys:           fsys,            // map fsys
		MatchFilesGlob: pkgio.YAMLMatch, // match only yaml files
		IgnoreRules:    ignoreRules,     // adhere to the ignore rules in the .kformignore file
		SkipDir:        true,            // a package is contained within a single directory, recursion is not needed
	}
	data, err := reader.Read(ctx, pkgio.NewData())
	if err != nil {
		fmt.Println("data read err", err.Error())
		return nil, nil, err
	}

	data.List(ctx, func(ctx context.Context, key store.Key, data []byte) {
		ko, err := fn.ParseKubeObject([]byte(data))
		if err != nil {
			fmt.Println("kubeObject parsing failed,", err.Error())
			recorder.Record(diag.DiagErrorf("kubeObject parsing failed, path: %s, err: %s", filepath.Join(path, key.Name), err.Error()))
			return
		}
		if ko.GetKind() == reflect.TypeOf(kformv1alpha1.KformFile{}).Name() {
			if kfile != nil {
				recorder.Record(diag.DiagErrorf("cannot have 2 kform file resource in the package"))
				return
			}
			kfKOE, err := koe.NewFromKubeObject[kformv1alpha1.KformFile](ko)
			if err != nil {
				recorder.Record(diag.DiagFromErr(err))
				return
			}
			kfile, err = kfKOE.GetGoStruct()
			if err != nil {
				recorder.Record(diag.DiagFromErr(err))
				return
			}
		} else {
			//ko.GetAnnotations() TBD do we need to look at annotations
			if input {
				ko.SetAnnotation(kformv1alpha1.KformAnnotationKey_BLOCK_TYPE, kformv1alpha1.BlockTYPE_INPUT.String())
			}
			kforms[filepath.Join(path, key.Name)] = ko
			log.Debug("read kubeObject", "fileName", key.Name, "kind", ko.GetKind(), "name", ko.GetName())
		}
	})
	fmt.Println("kubeObject parsing succeeded,")

	return kfile, kforms, nil
}
