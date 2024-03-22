/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parser

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// listProviderConfigs list the provider configs of the root package references from all resources in
// all packages
func (r *KformParser) GetProviderConfigs(ctx context.Context) (store.Storer[types.Block], sets.Set[string], error) {
	rootPackage, err := r.GetRootPackage(ctx)
	if err != nil {
		return nil, nil, err
	}
	providerConfigs := memory.NewStore[types.Block]()
	providerConfigSets := sets.New[string]()
	rootProviderConfigs := rootPackage.ListProviderConfigs(ctx)
	// walk through all packages and for all mixins list the providers
	// referenced by the resources.
	// we validate the provider config exists in the root package
	for _, pkg := range r.ListPackages(ctx) {
		if pkg.Kind != types.PackageKind_MIXIN {
			for _, providerName := range pkg.ListProvidersFromResources(ctx).UnsortedList() {
				providerConfig, ok := rootProviderConfigs[providerName]
				if !ok {
					return nil, nil, err
				}
				providerConfigs.Create(ctx, store.ToKey(providerName), providerConfig)
				providerConfigSets = providerConfigSets.Insert(providerName)
			}
		}
	}
	return providerConfigs, providerConfigSets, nil
}

// list providers list the providers references w/o aliases from all resources in
// all packages
func (r *KformParser) listRawProviders(ctx context.Context) (sets.Set[string], error) {
	rootPackage, err := r.GetRootPackage(ctx)
	if err != nil {
		return nil, err
	}
	providerSet := sets.New[string]()
	rootProviderConfigs := rootPackage.ListProviderConfigs(ctx)
	// walk through all packages and for all mixins list the raw providers
	// referenced by the resources.
	// we validate the provider config
	for _, pkg := range r.ListPackages(ctx) {
		if pkg.Kind != types.PackageKind_MIXIN {
			for _, provider := range pkg.ListRawProvidersFromResources(ctx).UnsortedList() {
				if _, ok := rootProviderConfigs[provider]; !ok {
					return nil, err
				}
				providerSet = providerSet.Insert(provider)
			}
		}
	}
	return providerSet, nil
}

// initialize the raw providers for which multiple instances could be instantiated
// e.g. for aliasing
func (r *KformParser) InitProviders(ctx context.Context) (store.Storer[types.Provider], error) {
	providers := memory.NewStore[types.Provider]()

	rawProviders, err := r.listRawProviders(ctx)
	if err != nil {
		return nil, err
	}
	for _, providerName := range rawProviders.UnsortedList() {
		providerEnv := fmt.Sprintf("KFORM_PROVIDER_%s", strings.ToUpper(providerName))
		providerExecPath, found := os.LookupEnv(providerEnv)
		if !found {
			return nil, fmt.Errorf("kform provider location has to be specified using env variable for now: %s", providerExecPath)
		}
		provider := types.Provider{}
		if err := provider.Init(ctx, providerExecPath, providerName); err != nil {
			return nil, err
		}
		providers.Create(ctx, store.ToKey(providerName), provider)
	}

	return providers, nil
}

func (r *KformParser) GetEmptyProviderInstances(ctx context.Context) (store.Storer[plugin.Provider], error) {
	providerInstances := memory.NewStore[plugin.Provider]()

	_, providerConfigNames, err := r.GetProviderConfigs(ctx)
	if err != nil {
		return nil, err
	}
	for _, providerConfigName := range providerConfigNames.UnsortedList() {
		providerInstances.Create(ctx, store.ToKey(providerConfigName), nil)
	}
	return providerInstances, nil
}
