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
