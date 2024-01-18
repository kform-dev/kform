package cctx

import "context"

func GetContextValue[T any](ctx context.Context, key any) T {
	x := ctx.Value(key)
	d, ok := x.(T)
	if !ok {
		var zero T
		return zero
	}
	return d
}
