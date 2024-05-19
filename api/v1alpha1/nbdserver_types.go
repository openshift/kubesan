// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type NbdServerSpec struct {
}

type NbdServerStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status

type NbdServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NbdServerSpec   `json:"spec,omitempty"`
	Status NbdServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type NbdServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NbdServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NbdServer{}, &NbdServerList{})
}
