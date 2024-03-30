package client

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestBuildObjMap(t *testing.T) {
	tests := map[string]struct {
		path                             string
		expectedProviders                []string
		expectedProviderConfigAPIVersion string
		expectedProviderConfigKind       string
		expectedPackages                 []string
		expectedPackageInventory         map[string]*invv1alpha1.PackageInventory
	}{
		"Empty": {
			path:                     "testfiles/inv1.yaml",
			expectedProviders:        []string{},
			expectedPackages:         []string{},
			expectedPackageInventory: nil,
		},
		"Single": {
			path:                             "testfiles/inv2.yaml",
			expectedProviders:                []string{"prov1"},
			expectedProviderConfigAPIVersion: "kubernetes.provider.kform.io/v1alpha1",
			expectedProviderConfigKind:       "ProviderConfig",
			expectedPackages:                 []string{"root"},
			expectedPackageInventory: map[string]*invv1alpha1.PackageInventory{
				"root": {
					PackageResources: map[string][]invv1alpha1.Object{
						"kubernetes_manifest.bla1": {
							{
								ObjectRef: invv1alpha1.ObjectReference{
									Group: "", Kind: "ConfigMap", Namespace: "default", Name: "cm1",
								},
							},
							{
								ObjectRef: invv1alpha1.ObjectReference{
									Group: "", Kind: "ConfigMap", Namespace: "default", Name: "cm2",
								},
							},
						},
					},
				},
			},
		},
		"Double": {
			path:                             "testfiles/inv3.yaml",
			expectedProviders:                []string{"prov1", "prov2"},
			expectedProviderConfigAPIVersion: "kubernetes.provider.kform.io/v1alpha1",
			expectedProviderConfigKind:       "ProviderConfig",
			expectedPackages:                 []string{"pkg1", "root"},
			expectedPackageInventory: map[string]*invv1alpha1.PackageInventory{
				"root": {
					PackageResources: map[string][]invv1alpha1.Object{
						"kubernetes_manifest.bla1": {
							{
								ObjectRef: invv1alpha1.ObjectReference{
									Group: "", Kind: "ConfigMap", Namespace: "default", Name: "cm1",
								},
							},
							{
								ObjectRef: invv1alpha1.ObjectReference{
									Group: "", Kind: "ConfigMap", Namespace: "default", Name: "cm2",
								},
							},
						},
					},
				},
				"pkg1": {
					PackageResources: map[string][]invv1alpha1.Object{
						"kubernetes_manifest.bla1": {
							{
								ObjectRef: invv1alpha1.ObjectReference{
									Group: "", Kind: "ConfigMap", Namespace: "default", Name: "cm1",
								},
							},
							{
								ObjectRef: invv1alpha1.ObjectReference{
									Group: "", Kind: "ConfigMap", Namespace: "default", Name: "cm2",
								},
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			inv := &unstructured.Unstructured{}
			b, err := os.ReadFile(tc.path)
			if err != nil {
				t.Errorf("unexpected error: %s", err.Error())
				return
			}
			if err := yaml.Unmarshal(b, inv); err != nil {
				t.Errorf("unexpected error: %s", err.Error())
				return
			}

			clusterInv := WrapInventoryObj(inv)
			storedInv, err := clusterInv.Load(ctx)
			if err != nil {
				t.Errorf("unexpected error: %s", err.Error())
				return
			}

			// validate providers
			providers := []string{}
			providerConfigs := map[string]*unstructured.Unstructured{}
			for provider, providerConfig := range storedInv.Providers {
				// append providers
				providers = append(providers, provider)
				// unmarshal providerConfig
				fmt.Println("providerConfig", providerConfig)
				provConfig := &unstructured.Unstructured{}
				if err := yaml.Unmarshal([]byte(providerConfig), provConfig); err != nil {
					t.Errorf("unexpected error: %s", err.Error())
					break
				}
				providerConfigs[provider] = provConfig
			}
			// sort strings for comparison
			sort.Strings(providers)
			if diff := cmp.Diff(providers, tc.expectedProviders); diff != "" {
				t.Errorf(diff)
				return
			}
			// check providerConfig content
			for _, providerConfig := range providerConfigs {
				if diff := cmp.Diff(providerConfig.GetAPIVersion(), tc.expectedProviderConfigAPIVersion); diff != "" {
					t.Errorf(diff)
				}
				if diff := cmp.Diff(providerConfig.GetKind(), tc.expectedProviderConfigKind); diff != "" {
					t.Errorf(diff)
				}
			}
			// validate packages
			packages := []string{}
			packageInventories := map[string]*invv1alpha1.PackageInventory{}
			for pkg, pkgInventory := range storedInv.Packages {
				// append providers
				packages = append(packages, pkg)
				// unmarshal providerConfig
				packageInventories[pkg] = pkgInventory
			}
			// sort strings for comparison
			sort.Strings(packages)
			if diff := cmp.Diff(packages, tc.expectedPackages); diff != "" {
				t.Errorf(diff)
				return
			}
			// check providerConfig content
			for pkg, pkgInventory := range packageInventories {
				if diff := cmp.Diff(pkgInventory, tc.expectedPackageInventory[pkg]); diff != "" {
					t.Errorf(diff)
				}
			}
		})
	}
}
