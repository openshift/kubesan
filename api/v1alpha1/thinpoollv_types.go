// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubesanslices "gitlab.com/kubesan/kubesan/internal/common/slices"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ThinPoolLvSpec struct {
	// Should be set from creation and never updated.
	VgName string `json:"vgName"`

	// May be updated at will.
	ThinLvs []ThinLvSpec `json:"thinLvs,omitempty"`
}

func (s *ThinPoolLvSpec) FindThinLv(name string) *ThinLvSpec {
	for i := range s.ThinLvs {
		if s.ThinLvs[i].Name == name {
			return &s.ThinLvs[i]
		}
	}
	return nil
}

// Only considers ThinLvs that should be activated.
func (s *ThinPoolLvSpec) ThinLvPreferredNodes() []string {
	nodes := []string{}
	for _, thinLv := range s.ThinLvs {
		if thinLv.Activate && thinLv.PreferredNode != nil {
			nodes = kubesanslices.AppendUnique(nodes, *thinLv.PreferredNode)
		}
	}
	return nodes
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
	Activate bool `json:"activate"`

	// May be updated at will.
	PreferredNode *string `json:"preferredNode,omitempty"`
}

type ThinLvContents struct {
	// The LVM thin LV should initially be zeroed out.
	Empty *ThinLvContentsEmpty `json:"empty,omitempty"`

	// The LVM thin LV should initially be a copy of another LVM thin LV.
	Snapshot *ThinLvContentsSnapshot `json:"snapshot,omitempty"`
}

type ThinLvContentsEmpty struct {
}

type ThinLvContentsSnapshot struct {
	SourceThinLvName string `json:"sourceThinLvName"`
}

type ThinPoolLvStatus struct {
	// Whether the LVM thin pool LV has been created.
	Created bool `json:"created"`

	// The name of the node where the LVM thin pool LV is active, along with any active LVM thin LVs.
	ActiveOnNode *string `json:"activeOnNode,omitempty"`

	// The status of each LVM thin LV that currently exists int the LVM thin pool LV.
	ThinLvs []ThinLvStatus `json:"thinLvs,omitempty"`
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
	State ThinLvState `json:"state"`

	// The current size of the LVM thin LV.
	SizeBytes int64 `json:"sizeBytes"`
}

type ThinLvState struct {
	// The LVM thin LV is not active on any node.
	Inactive *ThinLvStateInactive `json:"inactive,omitempty"`

	// The LVM thin LV is active on the node where the LVM thin pool LV is active.
	Active *ThinLvStateActive `json:"active,omitempty"`
}

type ThinLvStateCreating struct {
}

type ThinLvStateInactive struct {
}

type ThinLvStateActive struct {
	// The path at which the LVM thin LV is available on the node where the LVM thin pool LV is active.
	Path string `json:"path"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status

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
