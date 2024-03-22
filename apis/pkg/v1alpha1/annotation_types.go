package v1alpha1

// kform attibutes
const (
	KformAnnotationKeyPrefix         = "kform.dev"
	KformAnnotationKey_BLOCK_TYPE    = KformAnnotationKeyPrefix + "/" + "block-type"
	KformAnnotationKey_RESOURCE_TYPE = KformAnnotationKeyPrefix + "/" + "resource-type"
	KformAnnotationKey_RESOURCE_ID   = KformAnnotationKeyPrefix + "/" + "resource-id"
	KformAnnotationKey_COUNT         = KformAnnotationKeyPrefix + "/" + "count"
	KformAnnotationKey_FOR_EACH      = KformAnnotationKeyPrefix + "/" + "for-each"
	KformAnnotationKey_DEPENDS_ON    = KformAnnotationKeyPrefix + "/" + "depends-on"
	KformAnnotationKey_DEFAULT       = KformAnnotationKeyPrefix + "/" + "default"
	KformAnnotationKey_SOURCE        = KformAnnotationKeyPrefix + "/" + "source"
	KformAnnotationKey_VERSION       = KformAnnotationKeyPrefix + "/" + "version"
	KformAnnotationKey_DESCRIPTION   = KformAnnotationKeyPrefix + "/" + "description"
	KformAnnotationKey_SENSITIVE     = KformAnnotationKeyPrefix + "/" + "sensitive"
	KformAnnotationKey_LIFECYCLE     = KformAnnotationKeyPrefix + "/" + "lifecycle"
	KformAnnotationKey_PRECONDITION  = KformAnnotationKeyPrefix + "/" + "pre-condition"
	KformAnnotationKey_POSTCONDITION = KformAnnotationKeyPrefix + "/" + "post-condition"
	KformAnnotationKey_PROVIDERS     = KformAnnotationKeyPrefix + "/" + "providers"
	KformAnnotationKey_PROVIDER      = KformAnnotationKeyPrefix + "/" + "provider"
	KformAnnotationKey_PROVISIONER   = KformAnnotationKeyPrefix + "/" + "provisioner"
	KformAnnotationKey_ORGANIZATION  = KformAnnotationKeyPrefix + "/" + "organization"
	KformAnnotationKey_ALIAS         = KformAnnotationKeyPrefix + "/" + "alias"
	KformAnnotationKey_HOSTNAME      = KformAnnotationKeyPrefix + "/" + "hostname"
	KformAnnotationKey_PATH          = KformAnnotationKeyPrefix + "/" + "path"
	KformAnnotationKey_INDEX         = KformAnnotationKeyPrefix + "/" + "index"
)

var KformAnnotations = []string{
	KformAnnotationKey_BLOCK_TYPE,
	KformAnnotationKey_RESOURCE_TYPE,
	KformAnnotationKey_RESOURCE_ID,
	KformAnnotationKey_COUNT,
	KformAnnotationKey_FOR_EACH,
	KformAnnotationKey_DEPENDS_ON,
	KformAnnotationKey_DEFAULT,
	KformAnnotationKey_SOURCE,
	KformAnnotationKey_VERSION,
	KformAnnotationKey_DESCRIPTION,
	KformAnnotationKey_SENSITIVE,
	KformAnnotationKey_LIFECYCLE,
	KformAnnotationKey_PRECONDITION,
	KformAnnotationKey_POSTCONDITION,
	KformAnnotationKey_PROVIDERS,
	KformAnnotationKey_PROVIDER,
	KformAnnotationKey_PROVISIONER,
	KformAnnotationKey_ORGANIZATION,
	KformAnnotationKey_ALIAS,
	KformAnnotationKey_HOSTNAME,
	KformAnnotationKey_PATH,
	KformAnnotationKey_INDEX,
}
