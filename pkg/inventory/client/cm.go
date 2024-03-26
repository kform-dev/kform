package client

import (
	"context"
	"fmt"

	"github.com/henderiw/logger/log"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// WrapInventoryObj takes a passed ConfigMap,
// wraps it with the ConfigMap and upcasts the wrapper as
// an the Inventory interface.
func WrapInventoryObj(inv *unstructured.Unstructured) Storage {
	return &ConfigMap{inv: inv}
}

// WrapInventoryInfoObj takes a passed ConfigMap,
// wraps it with the ConfigMap and upcasts the wrapper as
// an the Info interface.
func WrapInventoryInfoObj(inv *unstructured.Unstructured) Info {
	return &ConfigMap{inv: inv}
}

func InvInfoToConfigMap(inv Info) *unstructured.Unstructured {
	r, ok := inv.(*ConfigMap)
	if ok {
		return r.inv
	}
	return nil
}

// ConfigMap wraps a ConfigMap resource and implements
// the Info/Storage interface.
type ConfigMap struct {
	inv *unstructured.Unstructured
}

var _ Info = &ConfigMap{}
var _ Storage = &ConfigMap{}

func (r *ConfigMap) Name() string {
	return r.inv.GetName()
}

func (r *ConfigMap) Namespace() string {
	return r.inv.GetNamespace()
}

func (r *ConfigMap) NamespacedName() string {
	return types.NamespacedName{Namespace: r.Namespace(), Name: r.Name()}.String()
}

func (r *ConfigMap) ID() string {
	// Empty string if not set.
	return r.inv.GetLabels()[invv1alpha1.InventoryLabelKey]
}

// Load is an Inventory interface function returning the set of
// object metadata per provider from the wrapped ConfigMap, or an error.
func (r *ConfigMap) Load(ctx context.Context) (*invv1alpha1.Inventory, error) {
	log := log.FromContext(ctx)
	storedInventory := &invv1alpha1.Inventory{}
	objMap, exists, err := unstructured.NestedStringMap(r.inv.Object, "data")
	if err != nil {
		return storedInventory, fmt.Errorf("error retrieving inventory, err: %s", err)
	}
	if exists {
		for key, value := range objMap {
			switch key {
			case "providers":
				storedInventory.Providers = map[string]string{}
				providers := map[string]string{}
				if err := yaml.Unmarshal([]byte(value), &providers); err != nil {
					log.Error("cannot unmarshal providers", "error", err.Error())
					return storedInventory, err
				}
				storedInventory.Providers = providers

			case "packages":
				packages := map[string]*invv1alpha1.PackageInventory{}
				if err := yaml.Unmarshal([]byte(value), &packages); err != nil {
					log.Error("cannot unmarshal packages", "error", err.Error())
					return storedInventory, err
				}
				storedInventory.Packages = packages
			default:
				// no need to fail, just log
				log.Debug("unexpected key")
			}
		}
	}
	return storedInventory, nil
}

// GetObject returns the wrapped object (ConfigMap) as a resource.Info
// or an error if one occurs.
func (icm *ConfigMap) GetObject() (*unstructured.Unstructured, error) {
	// Create the objMap of all the resources, and compute the hash.
	//objMap := buildObjMap(icm.objMetas, icm.objStatus)
	// Create the inventory object by copying the template.
	invCopy := icm.inv.DeepCopy()
	// Adds the inventory map to the ConfigMap "data" section.
	/*
		err := unstructured.SetNestedStringMap(invCopy.UnstructuredContent(),
			objMap, "data")
		if err != nil {
			return nil, err
		}
	*/
	return invCopy, nil
}
