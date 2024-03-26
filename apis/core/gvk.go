package core

import "k8s.io/apimachinery/pkg/runtime/schema"

var ConfigMapGVK = schema.GroupVersionKind{
	Group:   "",
	Kind:    "ConfigMap",
	Version: "v1",
}
