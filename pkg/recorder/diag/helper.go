package diag

import (
	"fmt"
	"time"

	"github.com/kform-dev/kform/pkg/recorder"
)

func Success(ctx string, start time.Time, d ...string) Diagnostic {
	// d is optional and allow the user to specify some detail on the success
	detail := ""
	if len(d) > 0 {
		detail = d[0]
	}
	return Diagnostic{
		Start:    start,
		Stop:     time.Now(),
		Severity: recorder.Severity_NONE,
		Context:  ctx,
		Detail:   detail,
	}
}

func DiagFromErr(err error) Diagnostic {
	if err == nil {
		return Diagnostic{}
	}
	return Diagnostic{
		Severity: recorder.Severity_ERROR,
		Detail:   err.Error(),
	}
}

func DiagFromErrWithContext(ctx string, err error) Diagnostic {
	if err == nil {
		return Diagnostic{}
	}
	return Diagnostic{
		Severity: recorder.Severity_ERROR,
		Detail:   err.Error(),
		Context:  ctx,
	}
}

func DiagErrorf(format string, a ...interface{}) Diagnostic {
	return Diagnostic{
		Severity: recorder.Severity_ERROR,
		Detail:   fmt.Sprintf(format, a...),
	}
}

func FromErrWithTimeContext(ctx string, start time.Time, err error) Diagnostic {
	return Diagnostic{
		Start:    start,
		Stop:     time.Now(),
		Severity: recorder.Severity_ERROR,
		Detail:   err.Error(),
		Context:  ctx,
	}
}

func DiagErrorfWithContext(ctx string, format string, a ...interface{}) Diagnostic {
	return Diagnostic{
		Severity: recorder.Severity_ERROR,
		Detail:   fmt.Sprintf(format, a...),
		Context:  ctx,
	}
}

func DiagWarnf(format string, a ...interface{}) Diagnostic {
	return Diagnostic{
		Severity: recorder.Severity_WARNING,
		Detail:   fmt.Sprintf(format, a...),
	}
}

func DiagWarnfWithContext(ctx string, format string, a ...interface{}) Diagnostic {
	return Diagnostic{
		Severity: recorder.Severity_WARNING,
		Detail:   fmt.Sprintf(format, a...),
		Context:  ctx,
	}
}
