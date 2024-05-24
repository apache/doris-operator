package v1

import "k8s.io/apimachinery/pkg/runtime/schema"

var SchemeGroupVersion = GroupVersion

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
