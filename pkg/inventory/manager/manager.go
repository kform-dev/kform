package manager

import (
	"context"
	"fmt"

	"github.com/henderiw/store"
	memstore "github.com/henderiw/store/memory"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"github.com/kform-dev/kform/pkg/inventory/client"
	"github.com/kform-dev/kform/pkg/inventory/config"
	"github.com/kform-dev/kform/pkg/inventory/policy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Manager interface {
	Apply(ctx context.Context) error
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
	//invInfo := client.WrapInventoryInfoObj(localInventory)
	//invStore := client.WrapInventoryObj(localInventory)

	// create a client to interact with the cluster backend
	client, err := client.ClusterClientFactory{StatusPolicy: policy.StatusPolicyNone}.NewClient(f)
	if err != nil {
		return nil, err
	}

	/*
		// get the stored inventory from the clusterBackend
		storedInventory, err := client.GetClusterInventory(ctx, invInfo)
		if err != nil {
			return nil, err
		}
	*/

	r := &manager{
		client:         client,
		localInventory: localInventory,
		//invStorage:        invStore,
		strategy:          strategy,
		storedProviders:   memstore.NewStore[string](),
		storedPackages:    memstore.NewStore[store.Storer[invv1alpha1.Object]](),
		actuatedProviders: memstore.NewStore[string](),
		actuatedPackages:  memstore.NewStore[store.Storer[invv1alpha1.Object]](),
	}

	/*
		for provider, providerConfig := range storedInventory.Providers {
			r.storedProviders.Create(ctx, store.ToKey(provider), providerConfig)
		}
		for pkg, packageInventory := range storedInventory.Packages {
			pkgResources := memstore.NewStore[invv1alpha1.Object]()
			r.storedPackages.Create(ctx, store.ToKey(pkg), pkgResources)

			for resource, objSet := range packageInventory.PackageResources {
				for idx, obj := range objSet {
					pkgResources.Create(
						ctx,
						store.KeyFromNSN(types.NamespacedName{
							Namespace: strconv.Itoa(idx),
							Name:      resource,
						}),
						obj,
					)
				}
			}
		}
	*/
	return r, nil
}

type manager struct {
	client            client.Client
	localInventory    *unstructured.Unstructured
	strategy          invv1alpha1.ActuationStrategy
	storedProviders   store.Storer[string] // string ia a yamlString
	storedPackages    store.Storer[store.Storer[invv1alpha1.Object]]
	actuatedProviders store.Storer[string] // string ia a yamlString
	actuatedPackages  store.Storer[store.Storer[invv1alpha1.Object]]
}

func (r *manager) Apply(ctx context.Context) error {
	// wrap the local inventory as a way to retrieve the inventory
	invStore := client.WrapInventoryObj(r.localInventory)
	inv, err := invStore.GetObject()
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
