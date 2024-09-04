package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Each operation that requires a thin-pool activation on a
// particular node is described by an Activation. Activations allow
// ThinPool to keep track of when it is safe to deactivate the
// thin-pool again.
type Activation struct {
	// Unique name (e.g. “delete-snapshot-<snapshot-name>”)
	Name string `json:"name"`

	// Which node the thin-pool should be attached to
	NodeName string `json:"nodeName"`
}

// ThinPoolSpec defines the desired state of ThinPool
type ThinPoolSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	VgName string `json:"vgName"`

	// Add an Activation onto this list in order to have the thin-pool
	// activated on the desired node eventually. If another Activation
	// is already in effect on another node, then this will only happen
	// when the existing Activation is removed from this list by its owner.
	Activations []Activation `json:"activations,omitempty"`
}

// ThinPoolStatus defines the observed state of ThinPool
type ThinPoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions
	// Available: The ThinPool was created and can be activated on nodes.
	// Activated: Indicates when an activation occurs.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []conditionsv1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Current Activations. There can be multiple Activations at the same
	// time if they have the same .NodeName.
	Activations []Activation `json:"activations,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ThinPool is the Schema for the thinpools API
type ThinPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThinPoolSpec   `json:"spec,omitempty"`
	Status ThinPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ThinPoolList contains a list of ThinPool
type ThinPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThinPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThinPool{}, &ThinPoolList{})
}
