/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package celrenderer

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/render2"
)

type CelRenderer interface {
	render2.Renderer
	RenderString(ctx context.Context, expr string) (any, error)
}

func New(varStore store.Storer[data.VarData], localVars map[string]any) CelRenderer {
	r := &renderer{
		localVars: localVars,
		varStore:  varStore,
	}
	r.Renderer = render2.New(r.RenderString)
	return r
}

type renderer struct {
	render2.Renderer
	localVars map[string]any
	varStore  store.Storer[data.VarData]
}

const specialCharExpr = "[$&+,:;=?@#|'<>\\-^*()%!]"

func isCelExpressionWithoutVariables(s string) (bool, error) {
	return regexp.MatchString(specialCharExpr, s)
}

func (r *renderer) isCelExression(ctx context.Context, expr string) bool {
	log := log.FromContext(ctx)
	for _, ref := range r.varStore.ListKeys() {
		if strings.Contains(expr, ref) {
			return true
		}
	}
	for ref := range r.localVars {
		if strings.Contains(expr, ref) {
			return true
		}
	}
	iscelexpr, err := isCelExpressionWithoutVariables(expr)
	if err != nil {
		log.Error("cel expression parsing failed", "error", err)
		return false
	}
	return iscelexpr
}

// getNewVars returns a new variable context with the local variables and the
// interested variables from the global variable store
func (r *renderer) getNewVars(ctx context.Context, expr string) (map[string]any, error) {
	log := log.FromContext(ctx)
	newVars := map[string]any{}
	r.varStore.List(func(key store.Key, vardata data.VarData) {
		if strings.Contains(expr, key.Name) {
			var v any
			var ok bool
			parts := strings.Split(key.Name, ".")
			if parts[0] == kformv1alpha1.BlockTYPE_PACKAGE.String() {
				if len(parts) != 3 {
					log.Error("wrong mixin name expecting module.<moduleName>.<outputname>", "got", key.Name)
					return
				}
				if v, ok = vardata[parts[2]]; !ok {
					log.Error("package variable does not exist in varStore", "ref", key.Name)
				}
			} else {
				if v, ok = vardata["BamBoozle"]; !ok {
					log.Error("package variable does not exist in varStore", "ref", key.Name)
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

func (r *renderer) RenderString(ctx context.Context, expr string) (any, error) {
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
		log.Debug("expression", "expr", expr)
		log.Debug("expression", "vars", newVars)
		env, err := getCelEnv(newVars)
		if err != nil {
			log.Error("cel environment failed", "error", err)
			return nil, err
		}
		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			log.Error("compile env to ast failed", "expr", expr, "error", iss.Err())
			return nil, err
		}
		_, err = cel.AstToCheckedExpr(ast)
		if err != nil {
			log.Error("ast to checked expression failed", "expr", expr, "error", err)
			return nil, err
		}
		prog, err := env.Program(ast,
			cel.EvalOptions(cel.OptOptimize),
			// TODO: uncomment after updating to latest k8s
			//cel.OptimizeRegex(library.ExtensionLibRegexOptimizations...),
		)
		if err != nil {
			log.Error("env program failed", "expr", expr, "error", err)
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
