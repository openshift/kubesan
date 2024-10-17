// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type DeviceSwitchSpec struct {
	Node      string  `json:"node"`               // should never change
	SizeBytes int64   `json:"sizeBytes"`          // may change at will
	InputURI  *string `json:"inputURI,omitempty"` // may change at will; supports file:// and nbd://
}

type DeviceSwitchStatus struct {
	Node       *string `json:"node"`
	OutputPath *string `json:"outputPath"`
	SizeBytes  *int64  `json:"sizeBytes"`
	InputURI   *string `json:"inputURI"`

	NbdDevice *string `json:"nbdDevice,omitempty"` // only relevant internally to the DeviceSwitch controller
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status

type DeviceSwitch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceSwitchSpec   `json:"spec,omitempty"`
	Status DeviceSwitchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type DeviceSwitchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceSwitch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeviceSwitch{}, &DeviceSwitchList{})
}
