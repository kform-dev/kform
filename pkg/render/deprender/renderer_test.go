package deprender

import (
	"context"
	"reflect"
	"testing"

	"sigs.k8s.io/yaml"
)

var f1 = `apiVersion: kpt.dev/v1
kind: Kptfile
metadata:
  name: xxx
  annotations:
    config.kubernetes.io/local-config: "true"
info:
  description: a.b.c
`

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		file    string
		deps    []string
		pkgdeps []string
	}{
		"Exists": {
			file:    f1,
			deps:    []string{"b.c"},
			pkgdeps: []string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var data map[string]any
			if err := yaml.Unmarshal([]byte(tc.file), &data); err != nil {
				t.Errorf("yaml unmarshal error: %s", err)
			}

			ctx := context.Background()

			renderer := New(tc.deps)
			if _, err := renderer.Render(ctx, data); err != nil {
				t.Errorf("render error: %s", err)
			}
			deps := renderer.GetDependencies(ctx)
			pkgdeps := renderer.GetPkgDependencies(ctx)

			if !reflect.DeepEqual(deps.UnsortedList(), tc.deps) {
				t.Errorf("want: %v, got: %v", tc.deps, deps)
			}
			if !reflect.DeepEqual(pkgdeps.UnsortedList(), tc.pkgdeps) {
				t.Errorf("want: %v, got: %v", tc.pkgdeps, pkgdeps)
			}
		})
	}
}
