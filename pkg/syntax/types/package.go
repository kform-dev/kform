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
	"github.com/kform-dev/kform/pkg/pkgio"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/render/deprender"
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
	for name := range ListBlocks(ctx, r.Blocks) {
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
	//fmt.Printf("json copy output %v\n", out)
	return out, nil
}

func (r *Package) AddDependencies(ctx context.Context) {
	blocks := r.ListBlocks(ctx)
	for blockName, block := range ListBlocks(ctx, r.Blocks, ListBlockOptions{PrexiExludes: []string{kformv1alpha1.BlockTYPE_INPUT.String()}}) {
		deprenderer := deprender.New(blocks)

		// we do this to avoid copying the data
		srcData, ok := block.GetData().Data[data.DummyKey]
		if !ok {
			r.recorder.Record(diag.DiagErrorf("block %s, does not have data for adding dependencies", blockName))
			continue
		}
		dstData, err := DeepCopy(srcData)
		if err != nil {
			r.recorder.Record(diag.DiagErrorf("block %s, does not have data for adding dependencies", blockName))
			continue
		}
		deprenderer.Render(ctx, dstData) // we need to avoid adding the Value here since the renderer adds nil values

		// TODO: add dependencies for attributes

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
	for name, b := range ListBlocks(ctx, r.Blocks, ListBlockOptions{PrexiExludes: []string{
		kformv1alpha1.BlockTYPE_INPUT.String(),
		kformv1alpha1.BlockTYPE_OUTPUT.String(),
		kformv1alpha1.BlockTYPE_LOCAL.String(),
		kformv1alpha1.BlockTYPE_PACKAGE.String(),
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
	Prefix       string
	PrexiExludes []string
}

func ListBlocks(ctx context.Context, s store.Storer[Block], opts ...ListBlockOptions) map[string]Block {
	blocks := map[string]Block{}
	s.List(ctx, func(ctx context.Context, key store.Key, data Block) {
		if opts != nil {
			if opts[0].Prefix != "" {
				if strings.HasPrefix(key.Name, opts[0].Prefix) {
					blocks[key.Name] = data
				}
			}
			if len(opts[0].PrexiExludes) != 0 {
				exluded := false
				for _, excludedPrefix := range opts[0].PrexiExludes {
					if strings.HasPrefix(key.Name, excludedPrefix) {
						exluded = true
						break
					}
				}
				if !exluded {
					blocks[key.Name] = data
				}
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

func (r *Package) ListProviderResources(ctx context.Context) sets.Set[string] {
	providers := sets.New[string]()
	for _, block := range ListBlocks(ctx, r.Blocks, ListBlockOptions{PrexiExludes: []string{
		kformv1alpha1.BlockTYPE_INPUT.String(),
		kformv1alpha1.BlockTYPE_OUTPUT.String(),
		kformv1alpha1.BlockTYPE_LOCAL.String(),
		kformv1alpha1.BlockTYPE_PACKAGE.String(),
		kformv1alpha1.BlockTYPE_PACKAGE.String(),
	}}) {
		providers.Insert(block.GetProvider())
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

func (r *Package) GenerateDAG(ctx context.Context, provider bool, unrefed []string) error {
	// add the vertices with the right VertexContext to the dag
	d, err := r.generateDAG(ctx, provider, unrefed)
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
func (r *Package) generateDAG(ctx context.Context, provider bool, unrefed []string) (dag.DAG[*VertexContext], error) {
	d := dag.New[*VertexContext]()

	d.AddVertex(ctx, dag.Root, &VertexContext{
		FileName:    filepath.Join(r.SourceDir, pkgio.PkgFileMatch[0]),
		PackageName: r.Name,
		BlockName:   r.Name,
		BlockType:   kformv1alpha1.BlockTYPE_ROOT,
	})

	if provider {
		// This is a providerDAG
		// Add inputs as the provider config might be depdendent on them
		for blockName, block := range ListBlocks(ctx, r.Blocks, ListBlockOptions{Prefix: kformv1alpha1.BlockTYPE_INPUT.String()}) {
			if err := addVertex(ctx, d, blockName, block); err != nil {
				return nil, err
			}

		}
		for providerName, block := range r.ListProviderConfigs(ctx) {
			// unreferenced provider configs should not be added to the dag
			found := false
			for _, name := range unrefed {
				if name == providerName {
					found = true
					break
				}
			}
			if !found {
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
		for blockName, block := range ListBlocks(ctx, r.Blocks) {
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

func (r *Package) GetBlockdata(ctx context.Context) map[string]any {
	blockData := map[string]any{}
	for blockName, block := range ListBlocks(ctx, r.Blocks) {
		blockData[blockName] = block.GetData()
	}
	return blockData
}
