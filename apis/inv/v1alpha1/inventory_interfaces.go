package v1alpha1

import (
	"context"
	"errors"

	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func (r Object) GetRnNode(blockType, resourceType, resourceID string) *yaml.RNode {
	rn := yaml.NewMapRNode(nil)
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

func MarshalProviders(providers map[string]string) ([]byte, error) {
	return yaml.Marshal(providers)
}

func MarshalPackages(ctx context.Context, pkgs store.Storer[store.Storer[data.BlockData]]) ([]byte, error) {
	packages := map[string]*PackageInventory{}
	var errm error
	pkgs.List(func(k store.Key, pkgStore store.Storer[data.BlockData]) {
		pkgName := k.Name
		packages[pkgName] = &PackageInventory{
			PackageResources: map[string][]Object{},
		}
		pkgStore.List(func(k store.Key, bd data.BlockData) {
			objs, err := getObject(bd)
			if err != nil {
				errors.Join(errm, err)
				return
			}
			packages[pkgName].PackageResources[k.Name] = objs
		})
	})
	if errm != nil {
		return nil, errm
	}
	return yaml.Marshal(packages)
}

func getObject(bd data.BlockData) ([]Object, error) {
	rns := bd.Get()
	objs := make([]Object, 0, len(rns))
	for _, rn := range rns {
		apiVersion := rn.GetApiVersion()
		gv, err := schema.ParseGroupVersion(apiVersion)
		if err != nil {
			return objs, err
		}
		objs = append(objs, Object{
			ObjectRef: ObjectReference{
				Group:     gv.Group,
				Version:   gv.Version,
				Kind:      rn.GetKind(),
				Name:      rn.GetName(),
				Namespace: rn.GetNamespace(),
			},
		})
	}
	return objs, nil
}
