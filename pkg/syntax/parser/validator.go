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
	"sort"
	"strings"

	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

// validateProviderConfigs validates if for each provider in a child resource
// there is a provider config
func (r *KformParser) validateProviderConfigs(ctx context.Context) {
	rootPackage, err := r.GetRootPackage(ctx)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
	}
	rootProviderConfigs := rootPackage.ListProviderConfigs(ctx)

	for packageName, pkg := range r.ListPackages(ctx) {
		if pkg.Kind != types.PackageKind_MIXIN {
			for _, provider := range pkg.ListProvidersFromResources(ctx).UnsortedList() {
				if _, ok := rootProviderConfigs[provider]; !ok {
					r.recorder.Record(diag.DiagErrorf("no provider config in root module for child module %s, provider: %s", packageName, provider))
				}
			}
		}
	}
}

func (r *KformParser) validateMixins(ctx context.Context) {
	for packageName, pkg := range r.ListPackages(ctx) {
		// only process packages  that mixin other packages
		mixins := types.ListBlocks(ctx, pkg.Blocks, types.ListBlockOptions{Prefix: kformv1alpha1.BlockTYPE_PACKAGE.String()})
		for mixinPackageName, mixin := range mixins {
			mixinPkg, err := r.packages.Get(ctx, store.ToKey(mixinPackageName))
			if err != nil {
				r.recorder.Record(diag.DiagErrorf("package mixin from %s to %s not found in package", packageName, mixinPackageName))
			}
			// validate if the module call matches the input of the remote module
			for inputName := range mixin.GetInputParameters() {
				inputName := fmt.Sprintf("input.%s", inputName)
				if _, err := mixinPkg.Blocks.Get(ctx, store.ToKey(inputName)); err != nil {
					r.recorder.Record(diag.DiagErrorf("package mixin from %s to %s not found in inputs %s", packageName, mixinPackageName, inputName))
				}
			}
			// validate the sourceproviders in the module call
			for targetProvider, sourceProvider := range mixin.GetProviders() {
				if _, err := pkg.ProviderConfigs.Get(ctx, store.ToKey(sourceProvider)); err != nil {
					r.recorder.Record(diag.DiagErrorf("provider package mixin from %s to %s source provider %s not found", packageName, mixinPackageName, sourceProvider))
				}
				if !mixinPkg.ListProvidersFromResources(ctx).Has(targetProvider) {
					r.recorder.Record(diag.DiagErrorf("provider package mixin from %s to %s target provider %s not found", packageName, mixinPackageName, targetProvider))
				}
			}
		}
		// validate remote call output
		// validate mod dependency matches with the remote module output
		for mixin, mixinCtx := range pkg.ListPkgDependencies(ctx) {
			split := strings.Split(mixin, ".")

			mixinPackageName := strings.Join([]string{split[0], split[1]}, ".")
			if _, ok := mixins[mixinPackageName]; !ok {
				r.recorder.Record(diag.DiagErrorf("package mixin from %s to %s not found in mixin fromctx: %s", packageName, mixinPackageName, mixinCtx))
			}
			mixinPkg, err := r.packages.Get(ctx, store.ToKey(mixinPackageName))
			if err != nil {
				r.recorder.Record(diag.DiagErrorf("package mixin from %s to %s not found in packages fromctx: %s", packageName, mixinPackageName, mixinCtx))
			}

			outputName := fmt.Sprintf("output.%s", split[2])
			if _, err := mixinPkg.Blocks.Get(ctx, store.ToKey(outputName)); err != nil {
				r.recorder.Record(diag.DiagErrorf("package mixin from %s to %s not found in outputs %s fromctx: %s", packageName, mixinPackageName, outputName, mixinCtx))
			}
		}
	}
}

func (r *KformParser) validateUnreferencedProviderConfigs(ctx context.Context) {
	unreferenceProviderConfigs := r.getUnReferencedProviderConfigs(ctx)
	if len(unreferenceProviderConfigs) > 0 {
		r.recorder.Record(diag.DiagWarnf("root module %s provider configs are unreferenced: %v", r.rootPackageName, unreferenceProviderConfigs))
	}
}

func (r *KformParser) getUnReferencedProviderConfigs(ctx context.Context) []string {
	unreferenceProviderConfigs := []string{}

	rootPackage, err := r.GetRootPackage(ctx)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
	}
	rootProviderConfigs := rootPackage.ListProviderConfigs(ctx)
	for _, pkg := range r.ListPackages(ctx) {
		for _, provider := range pkg.ListProvidersFromResources(ctx).UnsortedList() {
			delete(rootProviderConfigs, provider)
			if len(rootProviderConfigs) == 0 {
				return unreferenceProviderConfigs
			}
		}
	}
	if len(rootProviderConfigs) > 0 {
		unreferenceProviderConfigs := make([]string, 0, len(rootProviderConfigs))
		for providerConfigName := range rootProviderConfigs {
			unreferenceProviderConfigs = append(unreferenceProviderConfigs, providerConfigName)
		}
		sort.Strings(unreferenceProviderConfigs)
		return unreferenceProviderConfigs
	}
	return unreferenceProviderConfigs
}

func (r *KformParser) validateUnreferencedProviderRequirements(ctx context.Context) {
	rootPackage, err := r.GetRootPackage(ctx)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
	}

	for packageName, pkg := range r.ListPackages(ctx) {
		rootProviderReqs := pkg.ListProviderRequirements(ctx)
		for name := range rootPackage.ListProviderConfigs(ctx) {
			delete(rootProviderReqs, name)
			if len(rootProviderReqs) == 0 {
				continue
			}
		}
		if len(rootProviderReqs) > 0 {
			unreferenceProviderReqs := make([]string, 0, len(rootProviderReqs))
			for name := range rootProviderReqs {
				unreferenceProviderReqs = append(unreferenceProviderReqs, name)
			}
			sort.Strings(unreferenceProviderReqs)
			r.recorder.Record(diag.DiagWarnf("%s package %s provider requirements are unreferenced: %v", pkg.Kind, packageName, unreferenceProviderReqs))
		}
	}
}

// validateProviderRequirements validates if the source strings of all the provider
// requirements are consistent
// first we walk through all the provider requirements referenced by all modules
// per provider we check the consistency of the source address
func (r *KformParser) validateProviderRequirements(ctx context.Context) {
	for providerName, providerReq := range r.listProviderRequirements(ctx) {
		// per provider we check the consistency of the source address
		source := ""
		for _, req := range providerReq {
			if source != "" && source != req.Source {
				r.recorder.Record(diag.DiagErrorf("inconsistent provider requirements for %s source1: %s, source2: %s", providerName, source, req.Source))
			}
			source = req.Source
		}
	}
}

func (r *KformParser) listProviderRequirements(ctx context.Context) map[string][]kformv1alpha1.Provider {
	rootPackage, err := r.GetRootPackage(ctx)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
	}

	rootProviderConfigs := rootPackage.ListProviderConfigs(ctx)
	// delete the unreferenced provider configs from the provider configs
	unreferenceProviderConfigs := r.getUnReferencedProviderConfigs(ctx)
	for _, name := range unreferenceProviderConfigs {
		delete(rootProviderConfigs, name)
	}

	// we initialize all provider if they have aa req or not, if not the latest provider will be downloaded
	allprovreqs := map[string][]kformv1alpha1.Provider{}
	for nsn := range rootProviderConfigs {
		allprovreqs[nsn] = []kformv1alpha1.Provider{}
	}

	for _, pkg := range r.ListPackages(ctx) {
		provReqs := pkg.ListProviderRequirements(ctx)
		for providerName, provReq := range provReqs {
			if _, ok := rootProviderConfigs[providerName]; ok {
				// since we initialized allprovreqs we dont need to check if the list is initialized
				allprovreqs[providerName] = append(allprovreqs[providerName], provReq)
			}
		}
	}
	return allprovreqs
}
