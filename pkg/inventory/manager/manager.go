package manager

import (
	"context"
	"fmt"

	"github.com/henderiw/store"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/inventory/client"
	"github.com/kform-dev/kform/pkg/inventory/config"
	"github.com/kform-dev/kform/pkg/inventory/policy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Manager interface {
	GetInventory(ctx context.Context) (*invv1alpha1.Inventory, error)
	Apply(ctx context.Context, providers map[string]string, newActuatedResources store.Storer[store.Storer[data.BlockData]]) error
	Delete(ctx context.Context) error
	// AddProvider
	// AddPackage
	// AddResource
	// ActuateInventory
}

func New(ctx context.Context, path string, f util.Factory, strategy invv1alpha1.ActuationStrategy) (Manager, error) {
	// get the local inventory file, which serves as a reference to lookup
	// the inventory in the cluster backend
	localInventory, err := config.GetInventoryInfo(path)
	if err != nil {
		return nil, err
	}

	// create a client to interact with the cluster backend
	client, err := client.ClusterClientFactory{StatusPolicy: policy.StatusPolicyNone}.NewClient(f)
	if err != nil {
		return nil, err
	}

	r := &manager{
		client:         client,
		localInventory: localInventory,
		strategy:       strategy,
	}
	return r, nil
}

type manager struct {
	client         client.Client
	localInventory *unstructured.Unstructured
	strategy       invv1alpha1.ActuationStrategy
}

func (r *manager) Apply(ctx context.Context, providers map[string]string, newActuatedResources store.Storer[store.Storer[data.BlockData]]) error {
	// wrap the local inventory as a way to retrieve the inventory
	invStore := client.WrapInventoryObj(r.localInventory)
	inv, err := invStore.GetObject(ctx, providers, newActuatedResources)
	if err != nil {
		return err
	}
	if inv == nil {
		return fmt.Errorf("attempting to apply a nil inventory object")
	}
	return r.client.Apply(ctx, inv)
}

func (r *manager) GetInventory(ctx context.Context) (*invv1alpha1.Inventory, error) {
	invInfo := client.WrapInventoryInfoObj(r.localInventory)
	// get the stored inventory from the clusterBackend
	return r.client.GetClusterInventory(ctx, invInfo)
}

func (r *manager) Delete(ctx context.Context) error {
	return r.client.Delete(ctx, r.localInventory)
}
