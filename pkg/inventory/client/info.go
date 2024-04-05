package client

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// Info provides the methods to lookup the inventory object
type Info interface {
	// Namespace of the inventory object.
	// It should be the value of the field .metadata.namespace.
	Namespace() string

	// Name of the inventory object.
	// It should be the value of the field .metadata.name.
	Name() string

	// Combines Namespace and Name as a string
	NamespacedName() string

	// ID of the inventory object.
	ID() string
}

// ToUnstructuredFunc returns the unstructured object for the
// given Info.
type ToUnstructuredFunc func(Info) *unstructured.Unstructured
