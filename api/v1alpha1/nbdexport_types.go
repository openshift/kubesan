// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type NbdExportSpec struct {
	// The short name of the LV (Volume or Snapshot) to export, write-once
	// at creation.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	// +kubebuilder:validation:Pattern="[-a-z0-9]+"
	Export string `json:"export"`

	// The "/dev/..." path of the export, write-once at creation.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	// +kubebuilder:validation:Pattern="/dev/[-_/a-z0-9]+"
	Path string `json:"path"`

	// The node hosting the export. Write-once at creation.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	// +kubebuilder:validation:Pattern="[-_.a-z0-9]+"
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

	// NBD URI for connecting to the NBD export, using IP address.
	// write-once when Conditions["Available"] is first set
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	// + TODO Add TLS support, which changes this to a nbds:// URI
	// +kubebuilder:validation:Pattern="nbd://[0-9a-f:.]+/[-a-z0-9]+"
	Uri string `json:"uri,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Export",type=string,JSONPath=`.spec.export`,description='LV source of the export'
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`,description='Node hosting the export'
// + TODO determine if there is a way to print a column "Clients" that displays the number of items in the .spec.clients array
// +kubebuilder:printcolumn:name="Available",type=date,JSONPath=`.status.conditions[?(@.type=="Available")].lastTransitionTime`,description='Time since export was available'
// +kubebuilder:printcolumn:name="URI",type=string,JSONPath=`.status.uri`,description='NBD URI for the export'

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
