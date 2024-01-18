package celrender

import (
	"regexp"

	"github.com/google/cel-go/cel"
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
	return cel.NewEnv(opts...)
}
