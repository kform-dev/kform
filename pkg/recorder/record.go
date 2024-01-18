package recorder

type Record interface {
	GetSeverity() Severity
	GetDetail() string
	GetContext() string
	GetDetails() string
}

type Severity int

const (
	Severity_NONE Severity = iota
	Severity_WARNING
	Severity_ERROR
)

func (d Severity) String() string {
	return [...]string{"none", "warning", "error"}[d]
}
