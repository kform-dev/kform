package loader

/*
import (
	"context"
	"fmt"
	"reflect"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	koe "github.com/nephio-project/nephio/krm-functions/lib/kubeobject"
)


type kformloader struct {
	recorder recorder.Recorder[diag.Diagnostic]
}

func (r *kformloader) load(ctx context.Context, data store.Storer[[]byte], input bool) (*kformv1alpha1.KformFile, map[string]*fn.KubeObject, error) {
	log := log.FromContext(ctx)
	var kfile *kformv1alpha1.KformFile
	kforms := map[string]*fn.KubeObject{}
	recorder := r.recorder

	data.List(ctx, func(ctx context.Context, key store.Key, data []byte) {
		ko, err := fn.ParseKubeObject([]byte(data))
		if err != nil {
			fmt.Println("kubeObject parsing failed,", err.Error())
			recorder.Record(diag.DiagErrorf("kubeObject parsing failed, path: %s, err: %s", key.Name, err.Error()))
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
			kforms[key.Name] = ko
			log.Debug("read kubeObject", "fileName", key.Name, "kind", ko.GetKind(), "name", ko.GetName())
		}
	})
	return kfile, kforms, nil
}
*/
