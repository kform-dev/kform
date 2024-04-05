package runner

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/pkg/pkgio"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/parser/pkgparser"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func (r *runner) getInputVars(ctx context.Context) (map[string]any, error) {
	log := log.FromContext(ctx)
	inputRecorder := recorder.New[diag.Diagnostic]()
	ctx = context.WithValue(ctx, types.CtxKeyRecorder, inputRecorder)

	reader, err := r.getInputReader(ctx)
	if err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, nil
	}
	kformDataStore, err := reader.Read(ctx)
	if err != nil {
		return nil, err
	}
	inputParser, err := pkgparser.New(ctx, "inputParser")
	if err != nil {
		return nil, err
	}
	inputPkg := inputParser.Parse(ctx, kformDataStore)
	if inputRecorder.Get().HasError() {
		//inputRecorder.Print()
		log.Error("failed parsing input", "error", inputRecorder.Get().Error())
		return nil, inputRecorder.Get().Error()
	}
	//inputRecorder.Print()
	// initialize the input vars
	inputVars := map[string]any{}
	var errm error
	for v, bd := range inputPkg.GetBlockdata(ctx) {
		vardata, err := bd.GetVarData()
		if err != nil {
			errors.Join(errm, err)
			continue
		}
		inputVars[v] = vardata
	}
	return inputVars, nil

}

func (r *runner) getInputReader(_ context.Context) (pkgio.Reader[*yaml.RNode], error) {
	if r.cfg.InputData != nil {
		return &pkgio.KformMemReader{
			Data:  r.cfg.InputData,
			Input: true,
		}, nil
	} else {
		if r.cfg.Input != "" {
			fsi, err := os.Stat(r.cfg.Input)
			if err != nil {
				return nil, fmt.Errorf("cannot init kform, input path does not exist: %s", r.cfg.Input)
			}
			if !fsi.IsDir() {
				return &pkgio.KformFileReader{
					Path:  r.cfg.Input,
					Input: true,
				}, nil
			} else {
				return &pkgio.KformDirReader{
					Path:  r.cfg.Input,
					Input: true,
				}, nil
			}
		}
	}
	return nil, nil
}
