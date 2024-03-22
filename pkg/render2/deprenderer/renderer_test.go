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

package deprenderer

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var doc1 = `
# comment
apiVersion: v1 #comment
kind: ConfigMap
metadata:
  name: doc1
  namespace: default
data:
  description: a.b.c
`

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		input   string
		deps    []string
		pkgdeps []string
	}{
		"Exists": {
			input:   doc1,
			deps:    []string{"b.c"},
			pkgdeps: []string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			rn, err := yaml.Parse(tc.input)
			if err != nil {
				t.Errorf("yaml parse error: %s", err)
			}

			renderer := New(tc.deps)
			if _, err := renderer.Render(ctx, rn.YNode()); err != nil {
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

			fmt.Println(rn.MustString())
		})
	}
}
