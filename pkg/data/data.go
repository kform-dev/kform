package data

const DummyKey = "BamBoozle"

// BlockData contains the data of a block -> can be pre-processed or post-processed
type BlockData struct {
	// Data contains the data of the block.
	// For blockType package output we can have multiple key entries, so we store them using a key in the map
	// For all other blockTypes we use a dummy key
	Data map[string][]any
}

func NewBlockData() *BlockData {
	return &BlockData{Data: map[string][]any{}}
}

// Insert inserts data in the blockdata if you know the position
func (r *BlockData) Insert(key string, total, pos int, data any) {
	if slice, ok := r.Data[key]; !ok {
		r.Data[key] = make([]any, total)
	} else {
		// this is a bit weird but in this app it make sense
		// since the total amount is known within a run
		if len(slice) != total {
			r.Data[key] = make([]any, total)
		}
	}
	// Check if the position is out of bounds
	if pos < 0 || pos > len(r.Data[key]) {
		// Should never happen
		return
	}
	r.Data[key][pos] = data
}

func (r *BlockData) Add(key string, data any) {
	if _, ok := r.Data[key]; !ok {
		r.Data[key] = []any{}
	}
	r.Data[key] = append(r.Data[key], data)
}
