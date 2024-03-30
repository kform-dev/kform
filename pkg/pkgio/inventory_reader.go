package pkgio

import (
	"context"
	"fmt"
	"strings"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
)

type InventoryReader struct {
}

func (r *InventoryReader) Read(ctx context.Context, inv *invv1alpha1.Inventory) (store.Storer[[]byte], error) {
	datastore := memory.NewStore[[]byte]()
	// TODO add multi-package support
	for _, pkgInv := range inv.Packages {
		for resource, objects := range pkgInv.PackageResources {
			parts := strings.Split(resource, ".")
			if len(parts) != 2 {
				return datastore, fmt.Errorf("invalid resource should be <RESOURCE_TYPE>.<RESOURCE_ID>, got: %s", resource)
			}
			resourceType := parts[0]
			resourceID := parts[1]

			for _, obj := range objects {
				obj := obj
				// generates a yamlDoc from the obj
				rn := obj.GetRnNode(kformv1alpha1.BlockTYPE_DATA.String(), resourceType, resourceID)

				// we need to represent the resources as yaml files to please the kformReader
				datastore.Create(ctx, store.ToKey(fmt.Sprintf("%s.yaml", resource)), []byte(rn.MustString()))
			}
		}
	}
	for provider, config := range inv.Providers {
		// we need to represent the resources as yaml files to please the kformReader
		datastore.Create(ctx, store.ToKey(fmt.Sprintf("%s.yaml", provider)), []byte(config))
	}

	return datastore, nil

}
