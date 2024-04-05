package client

import (
	"context"

	"github.com/henderiw/store"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Storage describes methods necessary for an object which
// can persist the object metadata for pruning and other group
// operations.
type Storage interface {
	// GetObject returns the object that stores the inventory
	GetObject(ctx context.Context, providers map[string]string, newActuatedResources store.Storer[store.Storer[data.BlockData]]) (*unstructured.Unstructured, error)
	// Load retrieves the set of object metadata from the inventory object
	Load(ctx context.Context) (*invv1alpha1.Inventory, error)
}

// ToStorageFunc creates the object which implements the Storage
// interface from the passed inv object.
type ToStorageFunc func(*unstructured.Unstructured) Storage
