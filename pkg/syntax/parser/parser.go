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

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/pkgio"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/parser/pkgparser"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"github.com/kform-dev/kform/pkg/util/cctx"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Config struct {
	PackageName  string
	Path         string
	ResourceData store.Storer[[]byte]
}

// NewKformParser creates a new kform parser
// ctx: contains the recorder
// path: indicates the rootPath of the kform package
func NewKformParser(ctx context.Context, cfg *Config) (*KformParser, error) {
	recorder := cctx.GetContextValue[recorder.Recorder[diag.Diagnostic]](ctx, types.CtxKeyRecorder)
	if recorder == nil {
		return nil, fmt.Errorf("cannot parse without a recorder")
	}
	return &KformParser{
		cfg:             cfg,
		rootPackageName: fmt.Sprintf("%s.%s", kformv1alpha1.BlockTYPE_PACKAGE.String(), cfg.PackageName),
		recorder:        recorder,
		packages:        memory.NewStore[*types.Package](nil),
		//providers:      memory.NewStore[*address.Package](),
	}, nil
}

type KformParser struct {
	cfg *Config
	//rootPackagePath string
	rootPackageName string
	recorder        recorder.Recorder[diag.Diagnostic]
	packages        store.Storer[*types.Package]
}

func (r *KformParser) Parse(ctx context.Context) {
	// we start by parsing the root packages
	// if there are child packages/mixins they will be resolved concurrently
	//r.rootPackageName = fmt.Sprintf("%s.%s", kformv1alpha1.BlockTYPE_PACKAGE.String(), filepath.Base(r.rootPackagePath))
	r.parsePackage(ctx, r.rootPackageName, types.PackageKind_ROOT, r.cfg.Path, r.cfg.ResourceData)
	if r.recorder.Get().HasError() {
		return
	}

	r.validateProviderConfigs(ctx)
	r.validateMixins(ctx)
	r.validateUnreferencedProviderConfigs(ctx)
	r.validateUnreferencedProviderRequirements(ctx)
	r.validateProviderRequirements(ctx)
	r.validateBackend(ctx)

	// install providers
	r.validateAndOrInstallProviders(ctx)
	if r.recorder.Get().HasError() {
		return
	}

	_, usedProviderConfigs, err := r.GetProviderConfigs(ctx)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
		return
	}
	r.generateProviderDAG(ctx, usedProviderConfigs)
	r.generateDAG(ctx)
}

func (r *KformParser) parsePackage(ctx context.Context, packageName string, pkgType types.PackageKind, path string, data store.Storer[[]byte]) {
	ctx = context.WithValue(ctx, types.CtxKeyPackageName, packageName)
	//if r.rootPackagePath == path {
	ctx = context.WithValue(ctx, types.CtxKeyPackageKind, pkgType)
	//} else {
	//	ctx = context.WithValue(ctx, types.CtxKeyPackageKind, types.PackageKind_MIXIN)
	//}
	packageParser, err := pkgparser.New(ctx, r.cfg.PackageName)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
		return
	}
	// we either can get the data from a directory reader or a memory
	var kformDataStore store.Storer[*yaml.RNode]
	if data != nil {
		var err error
		reader := pkgio.KformMemReader{
			Data: data,
		}
		kformDataStore, err = reader.Read(ctx)
		if err != nil {
			r.recorder.Record(diag.DiagFromErr(err))
			return
		}
	} else {
		var err error
		reader := pkgio.KformDirReader{
			Path: path,
		}
		kformDataStore, err = reader.Read(ctx)
		if err != nil {
			r.recorder.Record(diag.DiagFromErr(err))
			return
		}
	}

	pkg := packageParser.Parse(ctx, kformDataStore)
	if r.recorder.Get().HasError() {
		// if an error is found we stop processing
		return
	}
	if err := r.packages.Create(store.ToKey(r.cfg.PackageName), pkg); err != nil {
		r.recorder.Record(diag.DiagErrorf("cannot create package %s", r.cfg.PackageName))
		return
	}

	// for each package that calls another package we need to continue
	// processing the new package -> these are mixins
	// TODO Mixin
	/*
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
	*/
}

func (r *KformParser) GetRootPackage(ctx context.Context) (*types.Package, error) {
	return r.packages.Get(store.ToKey(r.cfg.PackageName))
}

func (r *KformParser) ListPackages(ctx context.Context) map[string]*types.Package {
	packages := map[string]*types.Package{}
	r.packages.List(func(key store.Key, pkg *types.Package) {
		packages[key.Name] = pkg
	})
	return packages
}
