package policy

// StatusPolicy specifies whether the inventory client should apply status to
// the inventory object. The status contains the actuation and reconcile status
// of each object in the inventory.
//
//go:generate stringer -type=StatusPolicy -linecomment
type StatusPolicy int

const (
	// StatusPolicyNone disables inventory status updates.
	StatusPolicyNone StatusPolicy = iota // None

	// StatusPolicyAll fully enables inventory status updates.
	StatusPolicyAll // All
)
