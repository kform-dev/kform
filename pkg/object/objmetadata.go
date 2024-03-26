package object

import "k8s.io/apimachinery/pkg/runtime/schema"

// ObjMetadata identifies a KRM stored object
type ObjMetadata struct {
	Namespace string
	Name      string
	GroupKind schema.GroupKind
}
