package v1alpha1

import (
	"github.com/apparentlymart/go-versions/versions"
	"github.com/kform-dev/kform/pkg/syntax/address"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildKformFile returns a KformFile api type
func BuildKformFile(meta metav1.ObjectMeta, spec KformFileSpec) *KformFile {
	return &KformFile{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersion,
			Kind:       KformFileKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
	}
}

func (r Provider) Validate() error {
	if _, _, err := address.ParseSource(r.Source); err != nil {
		return err
	}
	if _, err := versions.MeetingConstraintsStringRuby(r.Version); err != nil {
		return err
	}
	return nil
}
