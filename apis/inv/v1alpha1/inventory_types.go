package v1alpha1

/*
data:
  providers:
	kubernetes:
	  config:
		apiVersion: kubernetes.provider.kform.dev/v1alpha1
		kind: ProviderConfig
		metadata:
			name: kubernetes
			namespace: default
			annotations:
				kform.dev/block-type: provider
		spec:
			configPath: "~/.kube/config"
  packages:
	root:
	  kubernetes_manifest.bla:
		resources:
		- ref:
		    group
			kind
			namespace
			name
		  status:
		    strategy:
			actuation:
			reconcile:
*/

// Non Goal: expose execution context
// Goal
// Expose the cluster resources that were applied to the system
// -> per provider track resources

type Inventory struct {
	Providers map[string]string            `json:"providers,omitempty" yaml:"providers,omitempty"`
	Packages  map[string]*PackageInventory `json:"packages,omitempty" yaml:"packages,omitempty"`
}

type PackageInventory struct {
	PackageResources map[string][]Object `json:",inline" yaml:",inline"`
}

type Object struct {
	ObjectRef ObjectReference `json:"objectRef,omitempty" yaml:"objectRef,omitempty"`
	// Strategy indicates the method of actuation (apply or delete) used or planned to be used.
	Strategy ActuationStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	// Actuation indicates whether actuation has been performed yet and how it went.
	Actuation ActuationStatus `json:"actuation,omitempty" yaml:"actuation,omitempty"`
	// Reconcile indicates whether reconciliation has been performed yet and how it went.
	Reconcile ReconcileStatus `json:"reconcile,omitempty" yaml:"reconcile,omitempty"`
}

// ObjectReference is a reference to a KRM resource by name and kind.
//
// Kubernetes only stores one API Version for each Kind at any given time,
// so version is not used when referencing objects.
type ObjectReference struct {
	// Group identifies an API namespace for REST resources.
	// If group is omitted, it is treated as the "core" group.
	// More info: https://kubernetes.io/docs/reference/using-api/#api-groups
	// +optional
	Group string `json:"group,omitempty" yaml:"group,omitempty"`

	// Version identifies an API Version for REST resources.
	Version string `json:"version,omitempty" version:"group,omitempty"`

	// Kind identifies a REST resource within a Group.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`

	// Name identifies an object instance of a REST resource.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Namespace identifies a group of objects across REST resources.
	// If namespace is specified, the resource must be namespace-scoped.
	// If namespace is omitted, the resource must be cluster-scoped.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

//nolint:revive // consistent prefix improves tab-completion for enums
//go:generate stringer -type=ActuationStrategy -linecomment
type ActuationStrategy int

const (
	ActuationStrategyApply  ActuationStrategy = iota // Apply
	ActuationStrategyDelete                          // Delete
)

//nolint:revive // consistent prefix improves tab-completion for enums
//go:generate stringer -type=ActuationStatus -linecomment
type ActuationStatus int

const (
	ActuationPending   ActuationStatus = iota // Pending
	ActuationSucceeded                        // Succeeded
	ActuationSkipped                          // Skipped
	ActuationFailed                           // Failed
)

//go:generate stringer -type=ReconcileStatus -linecomment
type ReconcileStatus int

const (
	ReconcilePending   ReconcileStatus = iota // Pending
	ReconcileSucceeded                        // Succeeded
	ReconcileSkipped                          // Skipped
	ReconcileFailed                           // Failed
	ReconcileTimeout                          // Timeout
)
