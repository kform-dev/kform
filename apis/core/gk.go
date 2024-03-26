package core

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var RBACGroupKind = map[schema.GroupKind]bool{
	{Group: rbacv1.GroupName, Kind: "Role"}:               true,
	{Group: rbacv1.GroupName, Kind: "ClusterRole"}:        true,
	{Group: rbacv1.GroupName, Kind: "RoleBinding"}:        true,
	{Group: rbacv1.GroupName, Kind: "ClusterRoleBinding"}: true,
}
