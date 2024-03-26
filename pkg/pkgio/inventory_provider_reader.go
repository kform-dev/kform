package pkgio

import (
	"context"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type InventoryProviderReader struct {
}

func (r *InventoryProviderReader) Read(ctx context.Context, providers map[string]string) (store.Storer[*yaml.RNode], error) {
	datastore := memory.NewStore[*yaml.RNode]()

	for provider, config := range providers {
		rn, err := yaml.Parse(config)
		if err != nil {
			return datastore, err
		}
		datastore.Create(ctx, store.ToKey(provider), rn)
	}
	return datastore, nil
}
