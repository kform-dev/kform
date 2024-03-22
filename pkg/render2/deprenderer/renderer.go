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
	"errors"
	"fmt"
	"strings"

	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/render2"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type DependencyRenderer interface {
	render2.Renderer
	RenderString(ctx context.Context, expr string) (any, error)
	ResolveDependsOn(ctx context.Context, rn *yaml.RNode) error
	GetDependencies(ctx context.Context) sets.Set[string]
	GetPkgDependencies(ctx context.Context) sets.Set[string]
}

func New(blocks []string) DependencyRenderer {
	r := &renderer{
		blocks:  blocks,
		deps:    sets.New[string](),
		pkgdeps: sets.New[string](),
	}
	r.Renderer = render2.New(r.RenderString)
	return r
}

type renderer struct {
	render2.Renderer
	blocks  []string
	deps    sets.Set[string]
	pkgdeps sets.Set[string] // we do 2 things but it allows to render the data once
}

func (r *renderer) GetDependencies(ctx context.Context) sets.Set[string] {
	return r.deps
}

func (r *renderer) GetPkgDependencies(ctx context.Context) sets.Set[string] {
	return r.pkgdeps
}

func (r *renderer) RenderString(ctx context.Context, expr string) (any, error) {
	for _, blockName := range r.blocks {
		if strings.Contains(expr, blockName) {
			r.deps.Insert(blockName)
			if strings.HasPrefix(blockName, kformv1alpha1.BlockTYPE_PACKAGE.String()) {
				r.pkgdeps.Insert(blockName)
			}
		}
	}
	return expr, nil
}

func (r *renderer) ResolveDependsOn(ctx context.Context, rn *yaml.RNode) error {
	// records all errors in the dependency
	var errm error
	// first check if depends_on annotation is present; if not nothing is to be done
	annotation := rn.GetAnnotations()
	if len(annotation) != 0 && annotation[kformv1alpha1.KformAnnotationKey_DEPENDS_ON] != "" {
		parts := strings.Split(annotation[kformv1alpha1.KformAnnotationKey_DEPENDS_ON], ",")
		// depends_on list dependencies as comma seperated strings
		// for each dependency listed

		for _, part := range parts {
			found := false
			for _, blockName := range r.blocks {
				if strings.Contains(part, blockName) {
					found = true
					break
				}
			}
			if found {
				r.deps.Insert(part)
				if strings.HasPrefix(part, kformv1alpha1.BlockTYPE_PACKAGE.String()) {
					r.pkgdeps.Insert(part)
				}
				continue
			}
			errors.Join(errm, fmt.Errorf("depends_on dependency %s not found for %s", part, rn.GetName()))
		}
	}
	return errm
}
