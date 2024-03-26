package v1alpha1

import (
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func (r Object) GetRnNode(blockType, resourceType, resourceID string) *yaml.RNode {
	rn := yaml.MakeNullNode()
	rn.SetApiVersion(schema.GroupVersion{Group: r.ObjectRef.Group, Version: r.ObjectRef.Version}.String())
	rn.SetKind(r.ObjectRef.Kind)
	rn.SetName(r.ObjectRef.Name)
	rn.SetNamespace(r.ObjectRef.Namespace)
	annotations := map[string]string{}
	annotations[kformv1alpha1.KformAnnotationKey_BLOCK_TYPE] = kformv1alpha1.BlockTYPE_DATA.String()
	annotations[kformv1alpha1.KformAnnotationKey_RESOURCE_TYPE] = resourceType
	annotations[kformv1alpha1.KformAnnotationKey_RESOURCE_ID] = resourceID
	rn.SetAnnotations(annotations)
	return rn
}
