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
	"fmt"
	"testing"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/data"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var input = `apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  namespace: network-system
  annotations:
    kform.dev/block-type: input
    kform.dev/resource-id: context ## this serves as a way to add default and manage the merge 
    kform.dev/default: true
data: 
  dataServer: 
    image: europe-docker.pkg.dev/srlinux/eu.gcr.io/data-server:latest
  configServerImage: europe-docker.pkg.dev/srlinux/eu.gcr.io/config-server:latest
`

var output = `apiVersion: kpt.dev/v1
apiVersion: apps/v1
kind: deployment
metadata:
  name: config-server
  namespace: network-system
  labels:
    config-server: "true"
spec:
  replicas: 1
  selector:
    matchLabels:
      config-server: "true"
  template:
    metadata:
      labels:
        config-server: "true"
    spec:
      serviceAccountName: config-server
      containers:
      - name: config-server
        image: input.context[0].data.configServerImage
        imagePullPolicy: always
        command:
        - /app/config-server
      - name: data-server
        image: input.context[0].data.dataServer.image
`

var celinput = `apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  namespace: default
  annotations:
    kform.dev/block-type: input
    kform.dev/resource-id: context ## this serves as a way to add default and manage the merge
    kform.dev/default: true
data:
  test: "a"
  strings: 
  - item1
  - item2
  - item3
`

var celoutput = `apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  namespace: default
  annotations:
    "kform.dev/for-each": "input.context[0].data.strings"
data:
  celnovar: "['input','context1'].concat('.')"  # cel fn w/o cell variables
  strings: "input.context[0].data.strings"
`

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		input  string
		output string
	}{
		"Basic": {
			input:  input,
			output: output,
		},
		"CellFunctions": {
			input:  celinput,
			output: celoutput,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			rni, err := yaml.Parse(tc.input)
			if err != nil {
				t.Errorf("yaml parse error: %s", err)
			}
			input := map[string]any{}
			yaml.Unmarshal([]byte(rni.MustString()), &input)

			rno, err := yaml.Parse(tc.output)
			if err != nil {
				t.Errorf("yaml parse error: %s", err)
			}

			varStore := memory.NewStore[data.VarData]()
			varStore.Create(ctx, store.ToKey("input.context"), map[string][]any{
				data.DummyKey: {input},
			})

			renderer := New(varStore, map[string]any{})
			out, err := renderer.Render(ctx, rno.YNode())
			if err != nil {
				t.Errorf("render error: %s", err)
			}
			rn := yaml.NewRNode(out)
			fmt.Println(rn.MustString())
		})
	}
}
