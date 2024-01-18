package types

import (
	"context"
	"fmt"

	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
)

var ResourceBlockTypes = []string{
	kformv1alpha1.BlockTYPE_RESOURCE.String(),
	kformv1alpha1.BlockTYPE_DATA.String(),
	kformv1alpha1.BlockTYPE_LIST.String(),
}

var BlockTypes = map[kformv1alpha1.BlockType]BlockInitializer{
	kformv1alpha1.BlockType_BACKEND:  newBackend,
	kformv1alpha1.BlockTYPE_PROVIDER: newProvider,
	kformv1alpha1.BlockTYPE_PACKAGE:  newMixin,
	kformv1alpha1.BlockTYPE_INPUT:    newInput,
	kformv1alpha1.BlockTYPE_OUTPUT:   newOutput,
	kformv1alpha1.BlockTYPE_LOCAL:    newLocal,
	kformv1alpha1.BlockTYPE_RESOURCE: newResource,
	kformv1alpha1.BlockTYPE_DATA:     newResource,
	kformv1alpha1.BlockTYPE_LIST:     newResource,
}

type BlockInitializer func(ctx context.Context) BlockProcessor

type BlockProcessor interface {
	UpdatePackage(context.Context)
}

func GetBlockTypeNames() []string {
	s := make([]string, 0, len(BlockTypes))
	for n := range BlockTypes {
		s = append(s, n.String())
	}
	return s
}

func InitializeBlock(ctx context.Context, blockType kformv1alpha1.BlockType) (BlockProcessor, error) {
	blockInitializer, ok := BlockTypes[blockType]
	if !ok {
		return nil, fmt.Errorf("cannot get blockType for %s, supported blocktypes %v", blockType.String(), GetBlockTypeNames())
	}
	return blockInitializer(ctx), nil
}
