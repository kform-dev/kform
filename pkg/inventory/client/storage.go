package client

import (
	"context"

	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Storage describes methods necessary for an object which
// can persist the object metadata for pruning and other group
// operations.
type Storage interface {
	// GetObject returns the object that stores the inventory
	GetObject() (*unstructured.Unstructured, error)
	// Load retrieves the set of object metadata from the inventory object
	Load(ctx context.Context) (*invv1alpha1.Inventory, error)
	// Store the set of object metadata in the inventory object. This will
	// replace the metadata, spec and status.
	//Store(objs object.ObjMetadataSet, status []actuation.ObjectStatus) error

	// Apply applies the inventory object. This utility function is used
	// in InventoryClient.Merge and merges the metadata, spec and status.
	//Apply(context.Context, dynamic.Interface, meta.RESTMapper, policy.StatusPolicy) error
	// ApplyWithPrune applies the inventory object with a set of pruneIDs of
	// objects to be pruned (object.ObjMetadataSet). This function is used in
	// InventoryClient.Replace. pruneIDs are required for enabling custom logic
	// handling of multiple ResourceGroup inventories.
	//ApplyWithPrune(context.Context, dynamic.Interface, meta.RESTMapper, StatusPolicy, object.ObjMetadataSet) error
}

// ToStorageFunc creates the object which implements the Storage
// interface from the passed inv object.
type ToStorageFunc func(*unstructured.Unstructured) Storage
