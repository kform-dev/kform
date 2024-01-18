package v1alpha1

import "github.com/google/cel-go/cel"

const (
	LoopAttrCount   = "count"
	LoopAttrForEach = "forEach"
	LoopKeyCount    = "count"
	LoopKeyEach     = "each"

	LoopKeyCountIndex = "count.index"
	LoopKeyForEachKey = "each.key"
	LoopKeyForEachVal = "each.value"
	LoopKeyItemsTotal = "items.total"
	LoopKeyItemsIndex = "items.index"
)

var LocalVars = map[string]struct{}{
	LoopKeyCountIndex: {},
	LoopKeyForEachKey: {},
	LoopKeyForEachVal: {},
}

var LoopAttr = map[string]*cel.Type{
	LoopAttrCount:   cel.IntType,
	LoopAttrForEach: cel.DynType,
}
