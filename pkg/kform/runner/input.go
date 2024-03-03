package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/henderiw/logger/log"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/loader"
	"github.com/kform-dev/kform/pkg/syntax/parser/pkgparser"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

func (r *runner) getInputVars(ctx context.Context) (map[string]any, error) {
	log := log.FromContext(ctx)
	inputRecorder := recorder.New[diag.Diagnostic]()
	ctx = context.WithValue(ctx, types.CtxKeyRecorder, inputRecorder)

	inputVars := map[string]any{}
	var kf *kformv1alpha1.KformFile
	var kforms map[string]*fn.KubeObject
	var err error
	if r.cfg.InputData != nil {
		kf, kforms, err = loader.KformMemoryLoader(ctx, r.cfg.InputData, true)
		if err != nil {
			return inputVars, err
		}
	} else {
		if r.cfg.Input != "" {
			fsi, err := os.Stat(r.cfg.Input)
			if err != nil {
				return inputVars, fmt.Errorf("cannot init kform, input path does not exist: %s", r.cfg.Input)
			}
			if !fsi.IsDir() {
				kf, kforms, err = loader.KformFileLoader(ctx, r.cfg.Input, true)
				if err != nil {
					return inputVars, err
				}
			} else {
				kf, kforms, err = loader.KformDirLoader(ctx, r.cfg.Input, true)
				if err != nil {
					return inputVars, err
				}
			}
		}
	}
	if len(kforms) != 0 {
		inputParser, err := pkgparser.New(ctx, "inputParser")
		if err != nil {
			return inputVars, err
		}
		inputPkg := inputParser.Parse(ctx, kf, kforms)
		if inputRecorder.Get().HasError() {
			inputRecorder.Print()
			log.Error("failed parsing input", "error", inputRecorder.Get().Error())
			return inputVars, inputRecorder.Get().Error()
		}
		inputRecorder.Print()
		// initialize the input vars
		inputVars = inputPkg.GetBlockdata(ctx)
	}
	return inputVars, nil
}
