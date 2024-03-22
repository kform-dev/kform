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

package render2

import (
	"context"
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Renderer interface {
	Render(ctx context.Context, node *yaml.Node) (*yaml.Node, error)
}

func New(stringFn StringFn) Renderer {
	return &walker{
		StringFn: stringFn,
	}
}

type StringFn func(ctx context.Context, x string) (any, error)

type walker struct {
	// StringFn is the function that parses the string
	StringFn
}

func (r *walker) Render(ctx context.Context, node *yaml.Node) (*yaml.Node, error) {
	var err error
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			node.Content[i+1], err = r.Render(ctx, node.Content[i+1])
			if err != nil {
				return nil, err
			}
		}
	case yaml.SequenceNode:
		for i, item := range node.Content {
			node.Content[i], err = r.Render(ctx, item)
			if err != nil {
				return nil, err
			}
		}
	case yaml.ScalarNode:
		if node.Tag == "!!str" { // Check if the scalar is a string
			if r.StringFn != nil {
				x, err := r.StringFn(ctx, node.Value)
				if err != nil {
					return node, nil
				}
				node.Tag = ""
				node.Value = fmt.Sprintf("%v", x)
			}
		}
	}
	return node, nil
}
