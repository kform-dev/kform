package pkgio

import (
	"context"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
)

type InventoryProviderReader struct {
}

func (r *InventoryProviderReader) Read(ctx context.Context, providers map[string]string) (store.Storer[[]byte], error) {
	datastore := memory.NewStore[[]byte]()

	for provider, config := range providers {

		datastore.Create(ctx, store.ToKey(provider), []byte(config))
	}
	return datastore, nil
}
