package diff

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Constants for masking sensitive values
const (
	sensitiveMaskDefault = "***"
	sensitiveMaskBefore  = "*** (before)"
	sensitiveMaskAfter   = "*** (after)"
)

// Masker masks sensitive values in an object while preserving diff-able
// changes.
//
// All sensitive values in the object will be masked with a fixed-length
// asterisk mask. If two values are different, an additional suffix will
// be added so they can be diff-ed.
type Masker struct {
	from *unstructured.Unstructured
	to   *unstructured.Unstructured
}

func NewMasker(from, to runtime.Object) (*Masker, error) {
	// Convert objects to unstructured
	f, err := toUnstructured(from)
	if err != nil {
		return nil, fmt.Errorf("convert to unstructured: %w", err)
	}
	t, err := toUnstructured(to)
	if err != nil {
		return nil, fmt.Errorf("convert to unstructured: %w", err)
	}

	// Run masker
	m := &Masker{
		from: f,
		to:   t,
	}
	if err := m.run(); err != nil {
		return nil, fmt.Errorf("run masker: %w", err)
	}
	return m, nil
}

// From returns the masked version of the 'from' object.
func (r *Masker) From() runtime.Object {
	return r.from
}

// To returns the masked version of the 'to' object.
func (r *Masker) To() runtime.Object {
	return r.to
}

// run compares and patches sensitive values.
func (m *Masker) run() error {
	// Extract nested map object
	from, err := dataFromUnstructured(m.from)
	if err != nil {
		return fmt.Errorf("extract 'data' field: %w", err)
	}
	to, err := dataFromUnstructured(m.to)
	if err != nil {
		return fmt.Errorf("extract 'data' field: %w", err)
	}

	for k := range from {
		// Add before/after suffix when key exists on both
		// objects and are not equal, so that it will be
		// visible in diffs.
		if _, ok := to[k]; ok {
			if from[k] != to[k] {
				from[k] = sensitiveMaskBefore
				to[k] = sensitiveMaskAfter
				continue
			}
			to[k] = sensitiveMaskDefault
		}
		from[k] = sensitiveMaskDefault
	}
	for k := range to {
		// Mask remaining keys that were not in 'from'
		if _, ok := from[k]; !ok {
			to[k] = sensitiveMaskDefault
		}
	}

	// Patch objects with masked data
	if m.from != nil && from != nil {
		if err := unstructured.SetNestedMap(m.from.UnstructuredContent(), from, "data"); err != nil {
			return fmt.Errorf("patch masked data: %w", err)
		}
	}
	if m.to != nil && to != nil {
		if err := unstructured.SetNestedMap(m.to.UnstructuredContent(), to, "data"); err != nil {
			return fmt.Errorf("patch masked data: %w", err)
		}
	}
	return nil
}

// dataFromUnstructured returns the underlying nested map in the data key.
func dataFromUnstructured(u *unstructured.Unstructured) (map[string]interface{}, error) {
	if u == nil {
		return nil, nil
	}
	data, found, err := unstructured.NestedMap(u.UnstructuredContent(), "data")
	if err != nil {
		return nil, fmt.Errorf("get nested map: %w", err)
	}
	if !found {
		return nil, nil
	}
	return data, nil
}

// toUnstructured converts a runtime.Object into an unstructured.Unstructured object.
func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, nil
	}
	c, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj.DeepCopyObject())
	if err != nil {
		return nil, fmt.Errorf("convert to unstructured: %w", err)
	}
	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(c)
	return u, nil
}
