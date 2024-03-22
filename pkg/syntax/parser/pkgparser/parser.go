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

package pkgparser

import (
	"context"
	"fmt"

	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"github.com/kform-dev/kform/pkg/util/cctx"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func New(ctx context.Context, packageName string) (*PackageParser, error) {
	recorder := cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, types.CtxKeyRecorder)
	if recorder == nil {
		return nil, fmt.Errorf("cannot parse without a recorder")
	}

	return &PackageParser{
		name:        cctx.GetContextValue[string](ctx, types.CtxKeyPackageName),
		kind:        cctx.GetContextValue[types.PackageKind](ctx, types.CtxKeyPackageKind),
		packageName: packageName,
		recorder:    recorder,
	}, nil
}

type PackageParser struct {
	name        string
	kind        types.PackageKind
	packageName string
	recorder    recorder.Recorder[diag.Diagnostic]
}

func (r *PackageParser) Parse(ctx context.Context, kformDataStore store.Storer[*yaml.RNode]) *types.Package {
	pkg := types.NewPackage(
		cctx.GetContextValue[string](ctx, types.CtxKeyPackageName),
		cctx.GetContextValue[types.PackageKind](ctx, types.CtxKeyPackageKind),
		r.recorder,
	)
	// TODO is kform file mandatory
	//r.recorder.Record(diag.DiagErrorf("cannot parse module with a kform file"))
	//return nil
	// add the required providers in the module
	/*
		if kf != nil {
			for providerRawName, providerReq := range kf.Spec.ProviderRequirements {
				if err := providerReq.Validate(); err != nil {
					r.recorder.Record(diag.DiagErrorf("cannot parse module provider requirement invalid for %s, err: %s", providerRawName, err.Error()))
					return nil
				}
				if err := pkg.ProviderRequirements.Create(
					ctx,
					store.ToKey(providerRawName),
					providerReq,
				); err != nil {
					r.recorder.Record(diag.DiagErrorf("cannot add provider %s in provider requirements, err: %s", providerRawName, err.Error()))
				}
			}
		}
	*/

	ctx = context.WithValue(ctx, types.CtxKeyPackage, pkg)
	r.validate(ctx, kformDataStore)
	if r.recorder.Get().HasError() {
		return nil
	}

	// now that we have all blocks we can check all the dependencies
	// we check if the name exists in the resource
	pkg.AddDependencies(ctx)
	if r.recorder.Get().HasError() {
		return nil
	}

	// resolve dependencies
	r.resolve(ctx, pkg)
	if r.recorder.Get().HasError() {
		return nil
	}

	return pkg
}
