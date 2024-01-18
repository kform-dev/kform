package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/loader"
	"github.com/kform-dev/kform/pkg/syntax/parser/pkgparser"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"github.com/kform-dev/kform/pkg/util/cctx"
)

// NewKformParser creates a new kform parser
// ctx: contains the recorder
// path: indicates the rootPath of the kform package
func NewKformParser(ctx context.Context, path string) (*KformParser, error) {
	recorder := cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, types.CtxKeyRecorder)
	if recorder == nil {
		return nil, fmt.Errorf("cannot parse without a recorder")
	}
	return &KformParser{
		rootPackagePath: path,
		recorder:        recorder,
		packages:        memory.NewStore[*types.Package](),
		//providers:      memory.NewStore[*address.Package](),
	}, nil
}

type KformParser struct {
	rootPackagePath string
	rootPackageName string
	recorder        recorder.Recorder[diag.Diagnostic]
	packages        store.Storer[*types.Package]
	//providers      store.Storer[*address.Package]
}

func (r *KformParser) Parse(ctx context.Context) {
	// we start by parsing the root packages
	// if there are child packages/mixins they will be resolved concurrently
	r.rootPackageName = fmt.Sprintf("%s.%s", kformv1alpha1.BlockTYPE_PACKAGE.String(), filepath.Base(r.rootPackagePath))
	r.parsePackage(ctx, r.rootPackageName, r.rootPackagePath)
	if r.recorder.Get().HasError() {
		return
	}

	r.validateProviderConfigs(ctx)
	r.validateMixins(ctx)
	r.validateUnreferencedProviderConfigs(ctx)
	r.validateUnreferencedProviderRequirements(ctx)
	r.validateProviderRequirements(ctx)

	// install providers
	r.validateAndOrInstallProviders(ctx)
	if r.recorder.Get().HasError() {
		return
	}

	r.generateProviderDAG(ctx, r.getUnReferencedProviderConfigs(ctx))
	r.generateDAG(ctx)
}

func (r *KformParser) parsePackage(ctx context.Context, packageName, path string) {
	ctx = context.WithValue(ctx, types.CtxKeyPackageName, packageName)
	if r.rootPackagePath == path {
		ctx = context.WithValue(ctx, types.CtxKeyPackageKind, types.PackageKind_ROOT)
	} else {
		ctx = context.WithValue(ctx, types.CtxKeyPackageKind, types.PackageKind_MIXIN)
	}
	packageParser, err := pkgparser.New(ctx, packageName)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
		return
	}

	// this is seperated to make the input vars more flexible
	kf, kforms, err := loader.GetKforms(ctx, path, false) // no special input processing requird
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
		return
	}

	pkg := packageParser.Parse(ctx, kf, kforms)
	if r.recorder.Get().HasError() {
		// if an error is found we stop processing
		return
	}
	if err := r.packages.Create(ctx, store.ToKey(packageName), pkg); err != nil {
		r.recorder.Record(diag.DiagErrorf("cannot create package %s", packageName))
		return
	}

	// for each package that calls another package we need to continue
	// processing the new package -> these are mixins

	mixins := map[store.Key]types.Block{}
	pkg.Blocks.List(ctx, func(ctx context.Context, key store.Key, block types.Block) {
		if strings.HasPrefix(key.Name, kformv1alpha1.BlockTYPE_PACKAGE.String()) {
			mixins[key] = block
		}

	})

	var wg sync.WaitGroup
	for key, mixin := range mixins {
		source := mixin.GetSource()
		path := fmt.Sprintf("./%s", filepath.Join(".", r.rootPackagePath, source))
		if _, err := os.Stat(path); err != nil {
			r.recorder.Record(diag.DiagErrorf("package %s, path %s does not exist", key.Name, path))
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.parsePackage(ctx, fmt.Sprintf("%s.%s", kformv1alpha1.BlockTYPE_PACKAGE.String(), filepath.Base(path)), path)
		}()
	}
	wg.Wait()
}

func (r *KformParser) GetRootPackage(ctx context.Context) (*types.Package, error) {
	return r.packages.Get(ctx, store.ToKey(r.rootPackageName))
}

func (r *KformParser) ListPackages(ctx context.Context) map[string]*types.Package {
	packages := map[string]*types.Package{}
	r.packages.List(ctx, func(ctx context.Context, key store.Key, pkg *types.Package) {
		packages[key.Name] = pkg
	})
	return packages
}
