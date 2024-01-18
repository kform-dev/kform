package diag

import (
	"fmt"
	"strings"
	"time"

	"github.com/kform-dev/kform/pkg/recorder"
)

var _ recorder.Record = &Diagnostic{}

type Diagnostic struct {
	Start    time.Time
	Stop     time.Time
	Severity recorder.Severity
	Detail   string
	Context  string
}

func (r Diagnostic) GetSeverity() recorder.Severity {
	return r.Severity
}

func (r Diagnostic) GetDetail() string {
	return r.Detail
}

func (r Diagnostic) GetContext() string {
	return r.Context
}

func (r Diagnostic) GetDetails() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("duration=%v", r.Stop.Sub(r.Start)))
	if r.Severity == recorder.Severity_NONE {
		sb.WriteString(", severity=SUCCESS")
	} else {
		sb.WriteString(fmt.Sprintf(", severity=%s", r.Severity))
	}

	sb.WriteString(fmt.Sprintf(", context=%s", r.Context))
	if r.Detail != "" {
		sb.WriteString(fmt.Sprintf(", detail=%s", r.Detail))
	}

	return sb.String()
}
