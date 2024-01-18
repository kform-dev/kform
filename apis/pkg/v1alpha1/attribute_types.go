package v1alpha1

type Attributes struct {
	APIVersion      string            `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind            string            `json:"kind,omitempty" yaml:"kind,omitempty"`
	ResourceType    string            `json:"resourceType,omitempty" yaml:"resourceType,omitempty"` // only relevant for resources
	ResourceID      string            `json:"resourceID,omitempty" yaml:"resourceID,omitempty"`     // only relevant for resources
	Count           string            `json:"count,omitempty" yaml:"count,omitempty"`
	ForEach         string            `json:"forEach,omitempty" yaml:"forEach,omitempty"`
	DependsOn       string            `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty"`
	Provider        string            `json:"provider,omitempty" yaml:"provider,omitempty"`
	Providers       map[string]string `json:"providers,omitempty" yaml:"providers,omitempty"` // only relevant for Mixin
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
	Sensitive       bool              `json:"sensitive,omitempty" yaml:"sensitive,omitempty"`
	Validation      string            `json:"validation,omitempty" yaml:"validation,omitempty"`
	LifeCycle       string            `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	PreCondition    string            `json:"preCondition,omitempty" yaml:"preCondition,omitempty"`
	PostCondition   string            `json:"postCondition,omitempty" yaml:"postCondition,omitempty"`
	Provisioner     string            `json:"provisioner,omitempty" yaml:"provisioner,omitempty"`
	Connection      string            `json:"connection,omitempty" yaml:"connection,omitempty"`
	HostName        string            `json:"hostName,omitempty" yaml:"hostName,omitempty"`
	Organization    string            `json:"organization,omitempty" yaml:"organization,omitempty"`
	Workspaces      map[string]string `json:"workspaces,omitempty" yaml:"workspaces,omitempty"`
	Source          string            `json:"source,omitempty" yaml:"source,omitempty"`
	Alias           string            `json:"alias,omitempty" yaml:"alias,omitempty"`
	InputParameters map[string]any    `json:"inputParameters,omitempty" yaml:"inputParameters,omitempty"` // only relevant for Mixin
}
