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

package types

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/dag"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/render2/deprenderer"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

func NewPackage(name string, kind PackageKind, recorder recorder.Recorder[diag.Diagnostic]) *Package {
	return &Package{
		Name:     name,
		Kind:     kind,
		recorder: recorder,

		ProviderRequirements: memory.NewStore[kformv1alpha1.Provider](),
		ProviderConfigs:      memory.NewStore[Block](),

		Blocks: memory.NewStore[Block](),
	}
}

type Package struct {
	Name      string
	Kind      PackageKind
	recorder  recorder.Recorder[diag.Diagnostic]
	SourceDir string

	Backend Block

	ProviderRequirements store.Storer[kformv1alpha1.Provider]
	ProviderConfigs      store.Storer[Block]

	Blocks store.Storer[Block]

	DAG         dag.DAG[*VertexContext]
	ProviderDAG dag.DAG[*VertexContext]
}

func (r *Package) ListBlocks(ctx context.Context) []string {
	blockNames := []string{}
	for name := range ListBlocks(ctx, r.Blocks, ListBlockOptions{
		ExludeOrphan: true,
	}) {
		blockNames = append(blockNames, name)
	}
	return blockNames
}

func DeepCopy(in any) (any, error) {
	if in == nil {
		return nil, errors.New("in cannot be nil")
	}
	bytes, err := json.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal input data")
	}
	var out interface{}
	err = json.Unmarshal(bytes, &out)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal to output data")
	}
	return out, nil
}

func (r *Package) AddDependencies(ctx context.Context) {
	// excludes the orphan blocks since these resources are there to avoid
	// kform from deleting them
	blocks := r.ListBlocks(ctx)
	// input blocks and orphan blocks should not have dependencies
	// so we can exclude them from finding dependencies
	for blockName, block := range ListBlocks(ctx, r.Blocks, ListBlockOptions{
		ExludeOrphan:  true,
		PrefixExludes: []string{kformv1alpha1.BlockTYPE_INPUT.String()},
	}) {
		deprenderer := deprenderer.New(blocks)
		// we do this to avoid copying the data
		for _, rn := range block.GetData().Get() {
			// This also includes the annotations as we validate all
			// the parameters in the object
			if _, err := deprenderer.Render(ctx, rn.YNode()); err != nil {
				r.recorder.Record(diag.DiagFromErr(err))
			}
			if err := deprenderer.ResolveDependsOn(ctx, rn); err != nil {
				r.recorder.Record(diag.DiagFromErr(err))
			}

		}
		block.UpdateDependencies(deprenderer.GetDependencies(ctx))
		block.UpdatePkgDependencies(deprenderer.GetPkgDependencies(ctx))
		r.Blocks.Update(ctx, store.ToKey(blockName), block)
	}
}

func (r *Package) ListPkgDependencies(ctx context.Context) sets.Set[string] {
	pkgDeps := sets.New[string]()
	for _, b := range ListBlocks(ctx, r.Blocks) {
		pkgDeps.Insert(b.GetPkgDependencies().UnsortedList()...)
	}
	return pkgDeps
}

func (r *Package) ResolveDAGDependencies(ctx context.Context) {
	for name, b := range ListBlocks(ctx, r.Blocks) {
		r.resolveDependencies(ctx, name, b)
	}
}

func (r *Package) resolveDependencies(ctx context.Context, name string, b Block) {
	for d, dctx := range b.GetDependencies() {
		switch strings.Split(d, ".")[0] {
		case kformv1alpha1.LoopKeyEach:
			if !b.HasForEach() {
				r.recorder.Record(diag.DiagErrorfWithContext(b.GetContext(name), "%s package: %s dependency resolution failed each requires a for_each attribute dependency: %s, ctx: %s", r.Kind.String(), r.Name, d, dctx))
			}
		case kformv1alpha1.LoopKeyCount:
			if !b.HasCount() {
				r.recorder.Record(diag.DiagErrorfWithContext(b.GetContext(name), "%s package: %s dependency resolution failed each requires a count attribute dependency: %s, ctx: %s", r.Kind.String(), r.Name, d, dctx))
			}
		default:
			if _, err := r.Blocks.Get(ctx, store.ToKey(d)); err != nil {
				r.recorder.Record(diag.DiagErrorfWithContext(b.GetContext(name), "%s package: %s dependency resolution failed for %s, ctx: %s, err: %s", r.Kind.String(), r.Name, d, dctx, err.Error()))
			}
		}
	}
}

func (r *Package) ResolveResource2ProviderConfig(ctx context.Context) {
	// list resources/data/etc
	for name, b := range ListBlocks(ctx, r.Blocks, ListBlockOptions{PrefixExludes: []string{
		kformv1alpha1.BlockTYPE_INPUT.String(),
		kformv1alpha1.BlockTYPE_OUTPUT.String(),
		kformv1alpha1.BlockTYPE_LOCAL.String(),
		kformv1alpha1.BlockTYPE_PACKAGE.String(),
	}}) {
		provider := b.GetProvider()
		if _, err := r.ProviderConfigs.Get(ctx, store.ToKey(provider)); err != nil {
			r.recorder.Record(diag.DiagErrorfWithContext(b.GetContext(name), "%s package: %s provider resolution resource2providerConfig failed for %s, err: %s", r.Kind.String(), r.Name, provider, err.Error()))
		}
	}
}

func (r *Package) ValidateMixinProviderConfigs(ctx context.Context) {
	if r.Kind == PackageKind_MIXIN {
		providerConfigs := []string{}
		r.ProviderConfigs.List(ctx, func(ctx context.Context, key store.Key, b Block) {
			providerConfigs = append(providerConfigs, key.Name)
		})
		if len(providerConfigs) > 0 {
			r.recorder.Record(diag.DiagErrorf("%s package: %s mixin packages cannot have provider configs, provider configs must come from the root module, providers: %v", r.Kind.String(), r.Name, providerConfigs))
		}
	}
}

type ListBlockOptions struct {
	Prefix        string
	PrefixExludes []string
	ExludeOrphan  bool
}

func ListBlocks(ctx context.Context, s store.Storer[Block], opts ...ListBlockOptions) map[string]Block {
	blocks := map[string]Block{}
	s.List(ctx, func(ctx context.Context, key store.Key, data Block) {
		if opts != nil {
			excluded := false
			if opts[0].Prefix != "" {
				if !strings.HasPrefix(key.Name, opts[0].Prefix) {
					excluded = true
				}
			}
			if len(opts[0].PrefixExludes) != 0 {
				for _, excludedPrefix := range opts[0].PrefixExludes {
					if strings.HasPrefix(key.Name, excludedPrefix) {
						excluded = true
						break
					}
				}
			}
			if opts[0].ExludeOrphan {
				if len(data.GetData().Get()) > 0 {
					annotations := data.GetData().Get()[0].GetAnnotations()
					if len(annotations) != 0 && annotations[kformv1alpha1.KformAnnotationKey_LIFECYCLE] != "" {
						excluded = true
					}
				}
			}
			if !excluded {
				// when the excludeOrphan is added we need to check for the lifecycle attribute
				// and exclude the object from the list
				blocks[key.Name] = data
			}

		} else {
			blocks[key.Name] = data
		}
	})
	return blocks
}

func (r *Package) ListProviderConfigs(ctx context.Context) map[string]Block {
	providerConfigs := map[string]Block{}
	r.ProviderConfigs.List(ctx, func(ctx context.Context, key store.Key, b Block) {
		providerConfigs[key.Name] = b
	})
	return providerConfigs
}

// ListProvidersFromResources lists all providers resource identified
// this includes direct provider mappings as well as aliases
func (r *Package) ListProvidersFromResources(ctx context.Context) sets.Set[string] {
	providers := sets.New[string]()
	for _, block := range ListBlocks(ctx, r.Blocks, ListBlockOptions{
		ExludeOrphan: true,
		PrefixExludes: []string{
			kformv1alpha1.BlockTYPE_INPUT.String(),
			kformv1alpha1.BlockTYPE_OUTPUT.String(),
			kformv1alpha1.BlockTYPE_LOCAL.String(),
			kformv1alpha1.BlockTYPE_PACKAGE.String(),
			kformv1alpha1.BlockTYPE_PROVIDER.String(),
		}}) {
		providers.Insert(block.GetProvider())
	}
	return providers
}

// ListRawProvidersFromResources lists all providers resource identified
// this includes only the main providers, no aliases
func (r *Package) ListRawProvidersFromResources(ctx context.Context) sets.Set[string] {
	providers := sets.New[string]()
	for _, block := range ListBlocks(ctx, r.Blocks, ListBlockOptions{
		ExludeOrphan: true,
		PrefixExludes: []string{
			kformv1alpha1.BlockTYPE_INPUT.String(),
			kformv1alpha1.BlockTYPE_OUTPUT.String(),
			kformv1alpha1.BlockTYPE_LOCAL.String(),
			kformv1alpha1.BlockTYPE_PACKAGE.String(),
			kformv1alpha1.BlockTYPE_PROVIDER.String(),
		}}) {
		providers.Insert(strings.Split(block.GetProvider(), "_")[0])
	}
	return providers
}

func (r *Package) ListProviderRequirements(ctx context.Context) map[string]kformv1alpha1.Provider {
	providerRequirements := map[string]kformv1alpha1.Provider{}
	r.ProviderRequirements.List(ctx, func(ctx context.Context, key store.Key, provider kformv1alpha1.Provider) {
		providerRequirements[key.Name] = provider
	})
	return providerRequirements
}

func (r *Package) GenerateDAG(ctx context.Context, provider bool, usedProviderConfigs sets.Set[string]) error {
	// add the vertices with the right VertexContext to the dag
	d, err := r.generateDAG(ctx, provider, usedProviderConfigs)
	if err != nil {
		return err
	}
	// connect the dag based on the depdenencies
	for n, v := range d.GetVertices() {
		deps := v.GetBlockDependencies()
		for dep := range deps {
			d.Connect(ctx, dep, n)
		}
		if n != dag.Root {
			if len(deps) == 0 {
				d.Connect(ctx, dag.Root, n)
			}
		}
	}
	// optimize the dag by removing the transitive connection in the dag
	d.TransitiveReduction(ctx)

	if provider {
		r.ProviderDAG = d
	} else {
		r.DAG = d
	}
	return nil
}
func (r *Package) generateDAG(ctx context.Context, provider bool, usedProviderConfigs sets.Set[string]) (dag.DAG[*VertexContext], error) {
	d := dag.New[*VertexContext]()

	d.AddVertex(ctx, dag.Root, &VertexContext{
		FileName:    filepath.Join(r.SourceDir, "provider"),
		PackageName: r.Name,
		BlockName:   r.Name,
		BlockType:   kformv1alpha1.BlockTYPE_ROOT,
	})

	if provider {
		// This is a providerDAG
		// Add inputs as the provider config might be depdendent on them
		for blockName, block := range ListBlocks(ctx, r.Blocks, ListBlockOptions{
			Prefix: kformv1alpha1.BlockTYPE_INPUT.String()}) {
			if err := addVertex(ctx, d, blockName, block); err != nil {
				return nil, err
			}
		}
		for providerName, block := range r.ListProviderConfigs(ctx) {
			// only add provider configs that are used to the dag
			if usedProviderConfigs.Has(providerName) {
				if err := addVertex(ctx, d, providerName, block); err != nil {
					return nil, err
				}
			}
		}
	} else {
		// this is NOT a provider DAG
		// for non provider DAGs we add the following blocktypes to the resources
		// blocktypes:
		// inputs
		// - outputs
		// - locals
		// - modules
		// - resources
		for blockName, block := range ListBlocks(ctx, r.Blocks, ListBlockOptions{ExludeOrphan: true}) {
			if err := addVertex(ctx, d, blockName, block); err != nil {
				return nil, err
			}
		}
	}
	return d, nil
}

func addVertex(ctx context.Context, d dag.DAG[*VertexContext], blockName string, block Block) error {
	d.AddVertex(ctx, blockName, &VertexContext{
		FileName:        block.GetFileName(),
		Index:           block.GetIndex(),
		PackageName:     block.GetPackageName(),
		BlockName:       blockName,
		BlockType:       block.GetBlockType(),
		Data:            block.GetData(),
		Attributes:      block.GetAttributes(),
		Dependencies:    block.GetDependencies(),
		PkgDependencies: block.GetPkgDependencies(),
	})
	return nil
}

func (r *Package) GetBlockdata(ctx context.Context) map[string]data.BlockData {
	blockData := map[string]data.BlockData{}
	for blockName, block := range ListBlocks(ctx, r.Blocks) {
		blockData[blockName] = block.GetData()
	}
	return blockData
}
