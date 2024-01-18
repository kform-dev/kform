package types

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"k8s.io/apimachinery/pkg/util/sets"
)

type blockValidator struct {
	recorder            recorder.Recorder[diag.Diagnostic]
	expectedAnnotations map[string]bool
}

var mandatory = true
var optional = false

func (r *blockValidator) validateAnnotations(ctx context.Context, ko *fn.KubeObject) {
	// copy expected annotations in annotationsSets to validate the presence
	annotationSets := sets.New[string]()
	for key := range r.expectedAnnotations {
		annotationSets.Insert(key)
	}

	// delete the annotations that are present
	// record warning for kform annotations that are present but ignored
	for annotionKey := range ko.GetAnnotations() {
		if strings.HasPrefix(annotionKey, kformv1alpha1.KformAnnotationKeyPrefix) {
			if !annotationSets.Has(annotionKey) {
				r.recorder.Record(diag.DiagWarnfWithContext(Context{ctx}.String(), "annotation %s present, but ignored", annotionKey))
			}
		}
	}

	// record errors for annotations that are not present and mandatory
	for _, annotionKey := range annotationSets.UnsortedList() {
		if mandatory, ok := r.expectedAnnotations[annotionKey]; !ok {
			if mandatory {
				r.recorder.Record(diag.DiagErrorfWithContext(Context{ctx}.String(), "mandatory annotation %s not present", annotionKey))
			}
		}
	}
}

// validateResourceSyntax validates the syntax of the resource kind
// resource Type must starts with a letter
// resource Type can container letters in lower and upper case, numbers and '-', '_'
func validateResourceSyntax(ctx context.Context, name string) error {
	re := regexp.MustCompile(`^[a-zA-Z]+[a-zA-Z0-9_-]*$`)
	if !re.Match([]byte(name)) {
		return fmt.Errorf("syntax error a resourceType starts with a letter and can contain letters in lower and upper case, numbers and '-', '_', got: %s", name)
	}
	return nil
}
