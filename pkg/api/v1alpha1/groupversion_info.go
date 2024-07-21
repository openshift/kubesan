// Package v1alpha1 contains API Schema definitions for the lvm v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=kubesan.gitlab.io
package v1alpha1

import (
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/config"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: config.Domain, Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &util.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
