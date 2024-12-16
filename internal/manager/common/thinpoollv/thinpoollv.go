// SPDX-License-Identifier: Apache-2.0
//
// Common ThinPoolLv code used by cluster and node controllers

package thinpoollv

import (
	"context"

	"github.com/kubesan/kubesan/api/v1alpha1"
	"github.com/kubesan/kubesan/internal/common/config"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Returns the thin LV name given the volume's name. This is necessary because
// thin-pool LV and thin LV names are in the same namespace and must be unique.
func VolumeToThinLvName(volumeName string) string {
	return volumeName + "-thin"
}

// Maps from ThinLvSpecState.Name to ThinLvStatusState.Name
func SpecStateToStatusState(specStateName string) string {
	switch specStateName {
	case v1alpha1.ThinLvSpecStateNameInactive:
		return v1alpha1.ThinLvStatusStateNameInactive
	case v1alpha1.ThinLvSpecStateNameActive:
		return v1alpha1.ThinLvStatusStateNameActive
	case v1alpha1.ThinLvSpecStateNameRemoved:
		return v1alpha1.ThinLvStatusStateNameRemoved
	default:
		return ""
	}
}

// Returns true if the ThinPoolLv should be active on a node
func thinPoolLvNeedsActivation(thinPoolLv *v1alpha1.ThinPoolLv) bool {
	// Cases that have been considered:
	// 1. Thin LV creation
	// 2. Thin LV deletion
	// 3. Thin LV activation
	// 4. Thin LV extension
	//
	// Update this list when you change which cases are handled by this
	// function. That way it will be easier to identify what still needs to
	// be implemented. It is critical that this function detects all cases
	// requiring activation/deactivation, otherwise the thin-pool will not
	// be activated appropriately or may remain activated when it shouldn't
	// be!
	// TODO populating from contents (cloning)
	// TODO NBD

	for i := range thinPoolLv.Spec.ThinLvs {
		thinLvSpec := &thinPoolLv.Spec.ThinLvs[i]
		thinLvStatus := thinPoolLv.Status.FindThinLv(thinLvSpec.Name)

		// corresponding Status.ThinLvs[] element doesn't exist yet

		if thinLvStatus == nil {
			return true
		}

		// thin LV is undergoing a state transition (e.g. creation/deletion/activation/deactivation)

		if SpecStateToStatusState(thinLvSpec.State.Name) != thinLvStatus.State.Name {
			return true
		}

		// thin LV is explicitly activated

		if thinLvSpec.State.Name == v1alpha1.ThinLvSpecStateNameActive {
			return true
		}

		// extending the thin LV requires that the ThinPoolLv be active on a node

		if thinLvSpec.SizeBytes > thinLvStatus.SizeBytes {
			return true
		}
	}

	return false
}

// Check whether work is pending that needs thin-pool activation. If yes,
// assign a suitable node to thinPoolLv.Spec.ActiveOnNode (if it is not already
// assigned). If no, clear thinPoolLv.Spec.ActiveOnNode.
//
// This is a level-triggered operation. It must be called every time we start
// or stop using the ThinPoolLv because its controller does not process updates
// when thinPoolLv.Spec.ActiveOnNode is clear. The thin-pool must also be
// deactivated once work has finished so it doesn't stay activated on a node
// where they are not needed.
//
// Returns whether thinPoolLv was modified. The caller is expected to call
// client.Update() if thinPoolLv was modified.
func recalcActiveOnNode(thinPoolLv *v1alpha1.ThinPoolLv) bool {
	needsActivation := thinPoolLvNeedsActivation(thinPoolLv)

	if needsActivation && thinPoolLv.Spec.ActiveOnNode == "" {
		// TODO replace with better node selection policy?
		thinPoolLv.Spec.ActiveOnNode = config.LocalNodeName
		return true
	}

	if !needsActivation && thinPoolLv.Spec.ActiveOnNode != "" {
		thinPoolLv.Spec.ActiveOnNode = ""
		return true
	}

	return false
}

// Recalculate Spec.ActiveOnNode and invoke client.Update() for thinPoolLv. If
// needUpdate is true then update even when Spec.ActiveOnNode is unchanged.
func UpdateThinPoolLv(ctx context.Context, client client.Client, thinPoolLv *v1alpha1.ThinPoolLv, needUpdate bool) error {
	log := log.FromContext(ctx).WithValues("nodeName", config.LocalNodeName)

	if recalcActiveOnNode(thinPoolLv) {
		needUpdate = true
	}
	if !needUpdate {
		log.Info("ThinPoolLv update not needed")
		return nil
	}
	log.Info("Updating ThinPoolLv", "Spec.ActiveOnNode", thinPoolLv.Spec.ActiveOnNode)
	return client.Update(ctx, thinPoolLv)
}
