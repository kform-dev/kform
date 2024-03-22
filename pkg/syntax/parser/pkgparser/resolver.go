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
	"github.com/kform-dev/kform/pkg/syntax/types"
)

func (r *PackageParser) resolve(ctx context.Context, pkg *types.Package) {
	// check the resources in the relevant lists to see if the dependency existss
	pkg.ResolveDAGDependencies(ctx)

	// for each resource we should have a required provider in root/mixin packages
	if pkg.Kind == types.PackageKind_ROOT {
		// validate that for each provider in a resource there is a related provider config
		pkg.ResolveResource2ProviderConfig(ctx)
	} else {
		// a mixin package must not have provider configs
		pkg.ValidateMixinProviderConfigs(ctx)
	}
}
