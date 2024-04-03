package client

import (
	"context"
	"fmt"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/apis/core"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"github.com/kform-dev/kform/pkg/inventory/policy"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/yaml"
)

// Client expresses an interface for interacting with
// objects which store references to objects (inventory objects).
type Client interface {
	// GetClusterInventory returns the inventory, which consists of the providers with their
	// resp. configs and the packages with their respective objRefs;
	// or an error if one occurred.
	GetClusterInventory(ctx context.Context, inv Info) (*invv1alpha1.Inventory, error)
	// GetClusterInventoryInfo returns the cluster inventory object.
	GetClusterInventoryInfo(ctx context.Context, inv Info) (*unstructured.Unstructured, error)

	Apply(ctx context.Context, inv *unstructured.Unstructured) error

	Delete(ctx context.Context, inv *unstructured.Unstructured) error
}

// ClusterClient is a implementation of the
// Client interface.
type ClusterClient struct {
	dc                    dynamic.Interface
	discoveryClient       discovery.CachedDiscoveryInterface
	mapper                meta.RESTMapper
	statusPolicy          policy.StatusPolicy
	gvk                   schema.GroupVersionKind
	invToStorageFunc      ToStorageFunc
	invToUnstructuredFunc ToUnstructuredFunc
}

var _ Client = &ClusterClient{}

// NewClient returns a concrete implementation of the
// Client interface or an error.
func NewClient(
	factory util.Factory,
	invToStorageFunc ToStorageFunc,
	invToUnstructuredFunc ToUnstructuredFunc,
	statusPolicy policy.StatusPolicy) (*ClusterClient, error) {

	dc, err := factory.DynamicClient()
	if err != nil {
		return nil, err
	}
	mapper, err := factory.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	discoveryClient, err := factory.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	clusterClient := ClusterClient{
		dc:                    dc,
		discoveryClient:       discoveryClient,
		mapper:                mapper,
		statusPolicy:          statusPolicy,
		gvk:                   core.ConfigMapGVK,
		invToStorageFunc:      invToStorageFunc,
		invToUnstructuredFunc: invToUnstructuredFunc,
	}
	return &clusterClient, nil
}

func (r *ClusterClient) GetClusterInventory(ctx context.Context, inv Info) (*invv1alpha1.Inventory, error) {
	clusterInv, err := r.GetClusterInventoryInfo(ctx, inv)
	if err != nil {
		return &invv1alpha1.Inventory{}, fmt.Errorf("failed to read inventory from cluster: %w", err)
	}
	// When nothing got applied it is normal this is nil
	if clusterInv == nil {
		return &invv1alpha1.Inventory{}, nil
	}
	invStorage := r.invToStorageFunc(clusterInv)
	return invStorage.Load(ctx)
}

func (r *ClusterClient) GetClusterInventoryInfo(ctx context.Context, inv Info) (*unstructured.Unstructured, error) {
	log := log.FromContext(ctx)
	localInv := r.invToUnstructuredFunc(inv)
	if localInv == nil {
		return nil, fmt.Errorf("cannot retrieve cluster inventory object with nil local inventory")
	}

	mapping, err := r.getMapping(localInv)
	if err != nil {
		return nil, err
	}
	log.Debug("fetching inventory", "nsn", inv.NamespacedName())
	clusterInv, err := r.dc.Resource(mapping.Resource).Namespace(inv.Namespace()).
		Get(ctx, inv.Name(), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	return clusterInv, nil
}

// getMapping returns the RESTMapping for the provided resource.
func (r *ClusterClient) Apply(ctx context.Context, inv *unstructured.Unstructured) error {
	r.ensureKformNamespace(ctx)

	mapping, err := r.getMapping(inv)
	if err != nil {
		return err
	}
	// Create client to interact with cluster.
	namespacedClient := r.dc.Resource(mapping.Resource).Namespace(inv.GetNamespace())

	// Get cluster object, if exsists.
	clusterInvObj, err := namespacedClient.Get(ctx, inv.GetName(), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// Create cluster inventory object, if it does not exist on cluster.
	if clusterInvObj == nil {
		_, err = namespacedClient.Create(ctx, inv, metav1.CreateOptions{})
		return err
	}

	// Update the cluster inventory object instead.
	_, err = namespacedClient.Update(ctx, inv, metav1.UpdateOptions{})
	return err
}

// getMapping returns the RESTMapping for the provided resource.
func (r *ClusterClient) Delete(ctx context.Context, inv *unstructured.Unstructured) error {
	mapping, err := r.getMapping(inv)
	if err != nil {
		return err
	}
	// Create client to interact with cluster.
	namespacedClient := r.dc.Resource(mapping.Resource).Namespace(inv.GetNamespace())

	// Get cluster object, if exsists.
	if _, err := namespacedClient.Get(ctx, inv.GetName(), metav1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	//Delete the inventory
	return namespacedClient.Delete(ctx, inv.GetName(), metav1.DeleteOptions{})
}

// getMapping returns the RESTMapping for the provided resource.
func (r *ClusterClient) getMapping(obj *unstructured.Unstructured) (*meta.RESTMapping, error) {
	return r.mapper.RESTMapping(obj.GroupVersionKind().GroupKind(), obj.GroupVersionKind().Version)
}

const KformSystemNamespace = `
apiVersion: v1
kind: Namespace
metadata:
  name: kform-system
`

func (r *ClusterClient) ensureKformNamespace(ctx context.Context) error {
	var ns *unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(KformSystemNamespace), &ns); err != nil {
		return err
	}
	mapping, err := r.getMapping(ns)
	if err != nil {
		return err
	}
	client := r.dc.Resource(mapping.Resource)
	clusterNamespaceObj, err := client.Get(ctx, ns.GetName(), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if clusterNamespaceObj == nil {
		_, err := client.Create(ctx, ns, metav1.CreateOptions{})
		return err
	}
	_, err = client.Update(ctx, ns, metav1.UpdateOptions{})
	return err
}
