package celrender

import (
	"context"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/render"
)

type CelRenderer interface {
	render.Renderer
}

type renderer struct {
	render.Renderer
	localVars map[string]any
	dataStore *data.DataStore
}

func New(dataStore *data.DataStore, localVars map[string]any) CelRenderer {
	r := &renderer{
		localVars: localVars,
		dataStore: dataStore,
	}
	r.Renderer = render.New(r.renderFn, r.stringRenderer)
	return r
}

func (r *renderer) renderFn(ctx context.Context, x any) (any, error) {
	return r.Render(ctx, x)
}

func (r *renderer) isCelExression(ctx context.Context, expr string) bool {
	for _, ref := range r.dataStore.ListKeys(ctx) {
		if strings.Contains(expr, ref) {
			return true
		}
	}
	for ref := range r.localVars {
		if strings.Contains(expr, ref) {
			return true
		}
	}
	return false
}

func (r *renderer) getNewVars(ctx context.Context, expr string) (map[string]any, error) {
	log := log.FromContext(ctx)
	newVars := map[string]any{}
	r.dataStore.List(ctx, func(ctx context.Context, key store.Key, blockData *data.BlockData) {
		if strings.Contains(expr, key.Name) {
			var v any
			var ok bool
			parts := strings.Split(key.Name, ".")
			if parts[0] == kformv1alpha1.BlockTYPE_PACKAGE.String() {
				if len(parts) != 3 {
					log.Error("wrong mixin name expecting module.<moduleName>.<outputname>", "got", key.Name)
					return
				}
				if v, ok = blockData.Data[parts[2]]; !ok {
					log.Error("package variable does not exist in result output", "ref", key.Name)
				}
			} else {
				if v, ok = blockData.Data[data.DummyKey]; !ok {
					log.Error("package variable does not exist in result output", "ref", key.Name)
				}
			}

			newVars[key.Name] = v
		}
	})

	for ref, v := range r.localVars {
		newVars[ref] = v
	}
	return newVars, nil
}

func (r *renderer) stringRenderer(ctx context.Context, expr string) (any, error) {
	log := log.FromContext(ctx)

	if r.isCelExression(ctx, expr) {
		// get the variables from the expression
		varsForExpr, err := r.getNewVars(ctx, expr)
		if err != nil {
			return nil, err
		}
		// replace reference . to _ otherwise cell complains to do json lookups of a struct
		newVars := make(map[string]any, len(varsForExpr))
		for origVar, v := range varsForExpr {
			newVar := strings.ReplaceAll(origVar, ".", "_")
			expr = strings.ReplaceAll(expr, origVar, newVar)
			newVars[newVar] = v
		}
		//log.Info("expression", "expr", expr)
		//log.Info("expression", "vars", newVars)
		env, err := getCelEnv(newVars)
		if err != nil {
			log.Error("cel environment failed", "error", err)
			return nil, err
		}
		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			log.Error("compile env to ast failed", "error", iss.Err())
			return nil, err
		}
		_, err = cel.AstToCheckedExpr(ast)
		if err != nil {
			log.Error("ast to checked expression failed", "error", err)
			return nil, err
		}
		prog, err := env.Program(ast,
			cel.EvalOptions(cel.OptOptimize),
			// TODO: uncomment after updating to latest k8s
			//cel.OptimizeRegex(library.ExtensionLibRegexOptimizations...),
		)
		if err != nil {
			log.Error("env program failed", "expression", expr, "error", err)
			return nil, err
		}

		// replace the reference since cel does not deal with . for json references
		newVarsForExpr := map[string]any{}
		for k, v := range varsForExpr {
			newVarsForExpr[strings.ReplaceAll(k, ".", "_")] = v
		}

		val, _, err := prog.Eval(newVarsForExpr)
		if err != nil {
			log.Error("evaluate program failed", "expression", expr, "error", err)
			return nil, err
		}

		/*
			result, err := val.ConvertToNative(reflect.TypeOf(""))
			if err != nil {
				log.Error("value conversion failed", "error", iss.Err())
				return nil, err
			}

			s, ok := result.(string)
			if !ok {
				return nil, fmt.Errorf("expression returned non-string value: %v", result)
			}
		*/
		return val.Value(), nil
	}
	return expr, nil
}
