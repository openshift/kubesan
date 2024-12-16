// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ThinPoolLvSpec struct {
	// Should be set from creation and never updated.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	VgName string `json:"vgName"`

	// May be updated at will.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	// +listType=map
	// +listMapKey=name
	ThinLvs []ThinLvSpec `json:"thinLvs,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Name of node where activation is needed, or empty.
	// When changing, may only toggle between "" and non-empty.
	// +kubebuilder:validation:XValidation:rule=(oldSelf==self)||((oldSelf=="")!=(self==""))
	ActiveOnNode string `json:"activeOnNode,omitempty"`
}

func (s *ThinPoolLvSpec) FindThinLv(name string) *ThinLvSpec {
	for i := range s.ThinLvs {
		if s.ThinLvs[i].Name == name {
			return &s.ThinLvs[i]
		}
	}
	return nil
}

type ThinLvSpec struct {
	// Should be set from creation and never updated.
	Name string `json:"name"`

	// Should be set from creation and never updated.
	Contents ThinLvContents `json:"contents"`

	// Should be set from creation and never updated.
	ReadOnly bool `json:"readOnly"`

	// Must be positive and a multiple of 512. May be updated at will, but the LVM thin LV's actual size will only
	// ever increase.
	SizeBytes int64 `json:"sizeBytes"`

	// May be updated at will.
	State ThinLvSpecState `json:"state"`
}

const (
	// The LVM thin LV is not active on any node.
	ThinLvSpecStateNameInactive = "Inactive"

	// The LVM thin LV is active on the node where the LVM thin pool LV is active.
	ThinLvSpecStateNameActive = "Active"

	// The LVM thin LV has been removed from the thin pool.
	ThinLvSpecStateNameRemoved = "Removed"
)

type ThinLvSpecState struct {
	// +unionDiscriminator
	// +kubebuilder:validation:Enum:="Inactive";"Active";"Removed"
	// +kubebuilder:validation:Required
	// + TODO add validation rule preventing transitions out of "Removed" state
	Name string `json:"name"`
}

const (
	// The LVM thin LV should initially be zeroed out.
	ThinLvContentsTypeEmpty = "Empty"

	// The LVM thin LV should initially be a copy of another LVM thin LV.
	ThinLvContentsTypeSnapshot = "Snapshot"
)

type ThinLvContents struct {
	// +unionDiscriminator
	// +kubebuilder:validation:Enum:="Empty";"Snapshot"
	// +kubebuilder:validation:Required
	ContentsType string `json:"contentsType"`

	// +optional
	Snapshot *ThinLvContentsSnapshot `json:"snapshot,omitempty"`
}

type ThinLvContentsSnapshot struct {
	SourceThinLvName string `json:"sourceThinLvName"`
}

const (
	ThinPoolLvConditionActive = "Active"
)

type ThinPoolLvStatus struct {
	// The generation of the spec used to produce this status.  Useful
	// as a witness when waiting for status to change.
	ObservedGeneration int64 `json:"observedGeneration"`

	// Conditions
	// Available: The LVM volume has been created
	// Active: The last time Status.ActiveOnNode changed
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []conditionsv1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// The name of the node where the LVM thin pool LV is active, along with any active LVM thin LVs; or "".
	// +optional
	ActiveOnNode string `json:"activeOnNode,omitempty"`

	// The status of each LVM thin LV that currently exists in the LVM thin pool LV.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	// +listType=map
	// +listMapKey=name
	ThinLvs []ThinLvStatus `json:"thinLvs,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

func (s *ThinPoolLvStatus) FindThinLv(name string) *ThinLvStatus {
	for i := range s.ThinLvs {
		if s.ThinLvs[i].Name == name {
			return &s.ThinLvs[i]
		}
	}
	return nil
}

type ThinLvStatus struct {
	// The name of the LVM thin LV.
	Name string `json:"name"`

	// The state of the LVM thin LV.
	State ThinLvStatusState `json:"state"`

	// The current size of the LVM thin LV.
	SizeBytes int64 `json:"sizeBytes"`
}

const (
	// The LVM thin LV is not active on any node.
	ThinLvStatusStateNameInactive = "Inactive"

	// The LVM thin LV is active on the node where the LVM thin pool LV is active.
	ThinLvStatusStateNameActive = "Active"

	// The LVM thin LV has been removed from the thin pool and can now be
	// forgotten by removing the corresponding Spec.ThinLvs[] element.
	ThinLvStatusStateNameRemoved = "Removed"
)

type ThinLvStatusState struct {
	// +unionDiscriminator
	// +kubebuilder:validation:Enum:="Inactive";"Active";"Removed"
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +optional
	Active *ThinLvStatusStateActive `json:"active,omitempty"`
}

type ThinLvStatusStateActive struct {
	// The path at which the LVM thin LV is available on the node where the LVM thin pool LV is active.
	Path string `json:"path"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=tp;tps;pool;pools,categories=kubesan
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="VG",type=string,JSONPath=`.spec.vgName`,description=`VG owning the thin pool`
// +kubebuilder:printcolumn:name="Activity",type=date,JSONPath=`.status.conditions[?(@.type=="Active")].lastTransitionTime`,description='Time since pool last changed activation status'
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.status.activeOnNode`,description='Node where thin pool is currently active'
// + TODO determine if there is a way to print a column "LVs" that displays the number of items in the .status.thinLvs array
// + TODO should we expose the thin pool size via Status?

type ThinPoolLv struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThinPoolLvSpec   `json:"spec,omitempty"`
	Status ThinPoolLvStatus `json:"status,omitempty"`
}

type ThinLv struct {
	Spec   *ThinLvSpec
	Status *ThinLvStatus
}

// Collects information about LVM thin LVs that are listed in the ThinPoolLv's Spec or Status (or both).
func (s *ThinPoolLv) ThinLvs() []ThinLv {
	thinLvs := []ThinLv{}

	// append ThinLvs with a Spec

	for i := range s.Spec.ThinLvs {
		spec := &s.Spec.ThinLvs[i]
		thinLvs = append(thinLvs, ThinLv{
			Spec:   spec,
			Status: s.Status.FindThinLv(spec.Name),
		})
	}

	// append ThinLvs with a Status but no Spec

	for i := range s.Status.ThinLvs {
		status := &s.Status.ThinLvs[i]
		if s.Spec.FindThinLv(status.Name) == nil {
			thinLvs = append(thinLvs, ThinLv{
				Spec:   nil,
				Status: status,
			})
		}
	}

	return thinLvs
}

// +kubebuilder:object:root=true

type ThinPoolLvList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThinPoolLv `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThinPoolLv{}, &ThinPoolLvList{})
}
