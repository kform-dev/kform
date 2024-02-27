package render

import (
	"context"
	"strings"

	"github.com/henderiw/logger/log"
)

type Renderer interface {
	Render(ctx context.Context, x any) (any, error)
}

func New(renderFn RenderFn, stringFn StringFn) Renderer {
	return &commonRenderer{
		RenderFn: renderFn,
		StringFn: stringFn,
	}
}

type RenderFn func(ctx context.Context, x any) (any, error)
type StringFn func(ctx context.Context, x string) (any, error)

type commonRenderer struct {
	RenderFn func(ctx context.Context, x any) (any, error)
	StringFn func(ctx context.Context, x string) (any, error)
}

func (r *commonRenderer) Render(ctx context.Context, x any) (any, error) {
	log := log.FromContext(ctx)
	var err error
	switch x := x.(type) {
	case map[string]any:
		for k, v := range x {
			if r.RenderFn != nil {
				x[k], err = r.RenderFn(ctx, v)
				if err != nil {
					log.Info("render map[string]any", "err", err.Error())
					// this is to handle cell rendering
					if strings.Contains(err.Error(), "no such key") || strings.Contains(err.Error(), "not found") {
						delete(x, k)
						continue
					} else {
						return nil, err
					}
				}
			}
		}
	case []any:
		newv := []any{}
		for i, v := range x {
			// rather then deleting the entry when the rendering failed harmlessly
			// -> add the entry to a new list (newx)
			if r.RenderFn != nil {
				newx, err := r.RenderFn(ctx, v)
				if err != nil {
					log.Info("render []any", "err", err.Error())
					if strings.Contains(err.Error(), "no such key") || strings.Contains(err.Error(), "not found") {
						x[i] = nil
						continue
					} else {
						return nil, err
					}
				}
				newv = append(newv, newx)
			}
		}
		return newv, nil
	case string:
		if r.StringFn != nil {
			return r.StringFn(ctx, x)
		}
	default:
	}
	return x, nil
}
