package fns

import (
	"context"
	"fmt"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn"
	"github.com/kform-dev/kform/pkg/render2/celrenderer"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func NewLocalOrOutputFn(cfg *Config) fn.BlockInstanceRunner {
	return &localOrOutput{
		rootPackageName: cfg.RootPackageName,
		varStore:        cfg.VarStore,
		outputStore:     cfg.OutputStore,
	}
}

type localOrOutput struct {
	rootPackageName string
	varStore        store.Storer[data.VarData]
	outputStore     store.Storer[data.BlockData]
}

func (r *localOrOutput) Run(ctx context.Context, vctx *types.VertexContext, localVars map[string]any) error {
	// NOTE: forEach or count expected and its respective values will be represented in localVars
	// ForEach: each.key/value
	// Count: count.index
	l := log.FromContext(ctx).With("vertexContext", vctx.String())
	ctx = log.IntoContext(ctx, l)
	log := log.FromContext(ctx)
	log.Debug("run block instance start...")
	// if the BlockContext Value is defined we render the expected output
	// the syntax parser should validate this, meaning the value should always be defined
	log.Debug("cel renderer start...")
	celrenderer := celrenderer.New(r.varStore, localVars)
	n, err := celrenderer.Render(ctx, vctx.Data.Get()[0].YNode()) // a copy is made for safety
	if err != nil {
		log.Error("cel renderer failed", "error", err)
		return err
	}
	log.Debug("cel renderer done...")
	rn := yaml.NewRNode(n)

	var v map[string]any
	if err := yaml.Unmarshal([]byte(rn.MustString()), &v); err != nil {
		return err
	}
	log.Debug("update varstore start...")
	if err := data.UpdateVarStore(ctx, r.varStore, vctx.BlockName, v, localVars); err != nil {
		log.Error("update varstore start failed", "error", err)
		return fmt.Errorf("update vars failed failed for blockName %s, err: %s", vctx.BlockName, err.Error())
	}
	log.Debug("update varstore done...")
	if vctx.BlockType == kformv1alpha1.BlockTYPE_OUTPUT {
		// add the path (fileName) and index annotiotn
		annotations := rn.GetAnnotations()
		annotations[kformv1alpha1.KformAnnotationKey_PATH] = vctx.FileName
		annotations[kformv1alpha1.KformAnnotationKey_INDEX] = vctx.Index
		rn.SetAnnotations(annotations)
		log.Debug("update blockStore start...")
		if err := data.UpdateBlockStoreEntry(ctx, r.outputStore, vctx.BlockName, rn, localVars); err != nil {
			return err
		}
		rn.SetAnnotations(annotations)
		log.Debug("update blockStore end...")
	}

	log.Debug("run block instance finished...")
	return nil
}
