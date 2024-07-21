package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Builder builds a new Scheme for mapping go types to Kubernetes GroupVersionKinds.
type Builder struct {
	GroupVersion schema.GroupVersion
	runtime.SchemeBuilder
}

// Register adds one or more objects to the SchemeBuilder so they can be added to a Scheme.  Register mutates bld.
func (bld *Builder) Register(object ...runtime.Object) *Builder {
	bld.SchemeBuilder.Register(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(bld.GroupVersion, object...)
		metav1.AddToGroupVersion(scheme, bld.GroupVersion)
		return nil
	})
	return bld
}

// RegisterAll registers all types from the Builder argument.  RegisterAll mutates bld.
func (bld *Builder) RegisterAll(b *Builder) *Builder {
	bld.SchemeBuilder = append(bld.SchemeBuilder, b.SchemeBuilder...)
	return bld
}

// AddToScheme adds all registered types to s.
func (bld *Builder) AddToScheme(s *runtime.Scheme) error {
	return bld.SchemeBuilder.AddToScheme(s)
}

// Build returns a new Scheme containing the registered types.
func (bld *Builder) Build() (*runtime.Scheme, error) {
	s := runtime.NewScheme()
	return s, bld.AddToScheme(s)
}
