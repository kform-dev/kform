package deprender

import (
	"context"
	"strings"

	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/render"
	"k8s.io/apimachinery/pkg/util/sets"
)

type DependencyRenderer interface {
	render.Renderer
	GetDependencies(ctx context.Context) sets.Set[string]
	GetPkgDependencies(ctx context.Context) sets.Set[string]
}

type renderer struct {
	render.Renderer
	blocks  []string
	deps    sets.Set[string]
	pkgdeps sets.Set[string] // we do 2 things but it allows to render the data once
}

func New(blocks []string) DependencyRenderer {
	r := &renderer{
		blocks:  blocks,
		deps:    sets.New[string](),
		pkgdeps: sets.New[string](),
	}
	r.Renderer = render.New(r.renderFn, r.stringRenderer)
	return r
}

func (r *renderer) GetDependencies(ctx context.Context) sets.Set[string] {
	return r.deps
}

func (r *renderer) GetPkgDependencies(ctx context.Context) sets.Set[string] {
	return r.pkgdeps
}

func (r *renderer) renderFn(ctx context.Context, x any) (any, error) {
	return r.Render(ctx, x)
}

func (r *renderer) stringRenderer(ctx context.Context, expr string) (any, error) {
	for _, blockName := range r.blocks {
		if strings.Contains(expr, blockName) {
			r.deps.Insert(blockName)
			if strings.HasPrefix(blockName, kformv1alpha1.BlockTYPE_PACKAGE.String()) {
				r.pkgdeps.Insert(blockName)
			}
		}
	}
	return nil, nil
}
