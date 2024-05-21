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
	"fmt"
	"regexp"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
)

const specialCharExpr = "[$&+,:;=?@#|'<>-^*()%!]"

func IsCelExpression(s string) (bool, error) {
	return regexp.MatchString(specialCharExpr, s)
}

func getCelEnv(vars map[string]any) (*cel.Env, error) {
	var opts []cel.EnvOption
	opts = append(opts, cel.HomogeneousAggregateLiterals())
	opts = append(opts, cel.EagerlyValidateDeclarations(true), cel.DefaultUTCTimeZone(true))
	//opts = append(opts, library.ExtensionLibs...)

	for k := range vars {
		// for builtin variables like count, forEach we know the type
		// this provide more type safety
		if ct, ok := kformv1alpha1.LoopAttr[k]; ok {
			opts = append(opts, cel.Variable(k, ct))
		} else {
			opts = append(opts, cel.Variable(k, cel.DynType))
		}
	}
	opts = append(opts, cel.Function("concat",
		cel.MemberOverload("string_concat",
			[]*cel.Type{cel.ListType(cel.StringType), cel.StringType},
			cel.StringType,
			cel.BinaryBinding(func(elems ref.Val, sep ref.Val) ref.Val {
				return stringOrError(concat(elems.(traits.Lister), string(sep.(types.String))))
			}),
		),
	))
	opts = append(opts, Lists())

	return cel.NewEnv(opts...)
}

func concat(strs traits.Lister, separator string) (string, error) {
	fmt.Println("wimconcat")
	sz := strs.Size().(types.Int)
	var sb strings.Builder
	for i := types.Int(0); i < sz; i++ {
		fmt.Println("wimconcat", i)
		if i != 0 {
			sb.WriteString(separator)
		}
		elem := strs.Get(i)
		str, ok := elem.(types.String)
		if !ok {
			fmt.Println("wimconcat", str)
			str = types.String(fmt.Sprintf("%v", elem))
		}
		sb.WriteString(string(str))
	}
	return sb.String(), nil
}

func stringOrError(str string, err error) ref.Val {
	if err != nil {
		return types.NewErr(err.Error())
	}
	return types.String(str)
}

func Lists() cel.EnvOption {
	return cel.Lib(listslib{})
}

type listslib struct{}

// LibraryName implements the SingletonLibrary interface method.
func (listslib) LibraryName() string {
	return "cel.lib.ext.lists"
}

// ProgramOptions implements the Library interface method.
func (listslib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// CompileOptions implements the Library interface method.
func (listslib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("listsconcat",
			cel.MemberOverload("lists_concat",
				[]*cel.Type{cel.AnyType, cel.AnyType},
				cel.ListType(cel.AnyType),
				cel.BinaryBinding(func(x1 ref.Val, x2 ref.Val) ref.Val {
					l1 := x1.(traits.Lister)
					l2 := x2.(traits.Lister)
					x2 = l1.Add(l2)
					return x2
				}),
			),
		),
	}
}
