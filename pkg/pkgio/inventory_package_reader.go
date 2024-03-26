package pkgio

import (
	"context"
	"fmt"
	"strings"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type InventoryReader struct {
}

func (r *InventoryReader) Read(ctx context.Context, pkgResources map[string][]invv1alpha1.Object) (store.Storer[*yaml.RNode], error) {
	datastore := memory.NewStore[*yaml.RNode]()

	for resource, objects := range pkgResources {
		parts := strings.Split(resource, ".")
		if len(parts) != 2 {
			return datastore, fmt.Errorf("invalid resource should be <RESOURCE_TYPE>.<RESOURCE_ID>, got: %s", resource)
		}
		resourceType := parts[0]
		resourceID := parts[1]
		for _, obj := range objects {
			// generates a yamlDoc from the obj
			rn := obj.GetRnNode(kformv1alpha1.BlockTYPE_DATA.String(), resourceType, resourceID)
			datastore.Create(ctx, store.ToKey(fmt.Sprintf("%s.%s", resource, rn.GetName())), rn)
		}
	}
	return datastore, nil
}
