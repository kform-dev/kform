package object

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// ProviderObjMetadataSet defines a set of Objects related to a providerConfig
type ProviderObjMetadataSet struct {
	Config  *unstructured.Unstructured
	Objects []ObjMetadata
}
