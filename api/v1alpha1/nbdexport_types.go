// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type NbdExportSpec struct {
	// The LV (Volume or Snapshot) that is being exported.
	// Should be set from creation and never updated.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	Source string `json:"source"`

	// The node hosting the export. Write-once at creation.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	Host string `json:"host"`

	// The set of clients connecting to the export.
	// +optional
	Clients []string `json:"clients,omitempty"`
}

type NbdExportStatus struct {
	// Conditions
	// Available: The export is currently accessible
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []conditionsv1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source`,description='LV source of the export'
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`,description='Node hosting the export'
// + TODO determine if there is a way to print a column "Clients" that displays the number of items in the .spec.clients array
// +kubebuilder:printcolumn:name="Available",type=date,JSONPath=`.status.conditions[?(@.type=="Available"&&@.status=="True")].lastTransitionTime`,description='Time since export was available'

type NbdExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NbdExportSpec   `json:"spec,omitempty"`
	Status NbdExportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NbdExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NbdExport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NbdExport{}, &NbdExportList{})
}
