package celrender

import (
	"context"
	"fmt"
	"testing"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/data"
	"sigs.k8s.io/yaml"
)

var input = `apiVersion: v1
kind: ConfigMap
metadata:
  name: context
  namespace: network-system
  annotations:
    kform.dev/block-type: input
    kform.dev/resource-id: context ## this serves as a way to add default and manage the merge 
    kform.dev/default: true
data: 
  data-server: 
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
        image: input.context[0].data.data-server.image
`

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		input  string
		output string
	}{
		"Test": {
			input:  input,
			output: output,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var input map[string]any
			if err := yaml.Unmarshal([]byte(tc.input), &input); err != nil {
				t.Errorf("yaml unmarshal error: %s", err)
			}
			var output map[string]any
			if err := yaml.Unmarshal([]byte(tc.output), &output); err != nil {
				t.Errorf("yaml unmarshal error: %s", err)
			}

			ctx := context.Background()

			dataStore := data.DataStore{Storer: memory.NewStore[*data.BlockData]()}
			dataStore.Create(ctx, store.ToKey("input.context"), &data.BlockData{
				Data: map[string][]any{
					data.DummyKey: {input},
				},
			})

			renderer := New(&dataStore, map[string]any{})
			out, err := renderer.Render(ctx, output)
			if err != nil {
				t.Errorf("render error: %s", err)
			}
			b, err := yaml.Marshal(out)
			if err != nil {
				t.Errorf("render error: %s", err)
			}
			fmt.Println(string(b))
		})
	}
}
