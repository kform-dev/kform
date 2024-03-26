package v1alpha1

const (
	// InventoryLabelKey is the label stored on the ConfigMap
	// inventory object. The value of the label is a unique
	// identifier (by default a UUID), representing the set of
	// objects applied at the same time as the inventory object.
	// This inventory object is used for pruning and deletion.
	InventoryLabelKey = "inv.kform.dev/inventory-id"

	// InventoryOwnerKey is the annotation key indicating the inventory owning an object.
	InventoryOwnerKey = "inv.kform.dev/inventory-owner"

	// LifecycleDeletionAnnotation is the lifecycle annotation key for deletion operation.
	//LifecycleDeleteAnnotation = "client.lifecycle.config.k8s.io/deletion"

	// PreventDeletion is the value used with LifecycleDeletionAnnotation
	// to prevent deleting a resource.
	//PreventDeletion = "detach"

	//OnRemoveAnnotation = "cli-utils.sigs.k8s.io/on-remove"
	// Resource lifecycle annotation value to prevent deletion.

	// Resource lifecycle annotation value to prevent deletion.
	//OnRemoveKeep = "keep"
)
