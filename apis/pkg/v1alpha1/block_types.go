package v1alpha1

import "fmt"

// GetBlockName returns the block name
func GetBlockName(a, b string) string {
	return fmt.Sprintf("%s.%s", a, b)
}

type BlockType int

const (
	BlockType_UNKNOWN BlockType = iota
	BlockType_BACKEND
	BlockType_REQUIREDPROVIDERS
	BlockTYPE_PROVIDER
	BlockTYPE_PACKAGE
	BlockTYPE_INPUT
	BlockTYPE_OUTPUT
	BlockTYPE_LOCAL
	BlockTYPE_RESOURCE
	BlockTYPE_DATA
	BlockTYPE_LIST
	BlockTYPE_ROOT
)

func (d BlockType) String() string {
	return [...]string{"unknown", "backend", "requiredProviders", "provider", "package", "input", "output", "local", "resource", "data", "list", "root"}[d]
}

func GetBlockType(n string) BlockType {
	switch n {
	case "backend":
		return BlockType_BACKEND
	case "requiredProviders":
		return BlockType_REQUIREDPROVIDERS
	case "provider":
		return BlockTYPE_PROVIDER
	case "package":
		return BlockTYPE_PACKAGE
	case "input":
		return BlockTYPE_INPUT
	case "output":
		return BlockTYPE_OUTPUT
	case "local":
		return BlockTYPE_LOCAL
	case "resource":
		return BlockTYPE_RESOURCE
	case "data":
		return BlockTYPE_DATA
	case "list":
		return BlockTYPE_LIST
	case "root":
		return BlockTYPE_ROOT
	default:
		return BlockType_UNKNOWN
	}
}
