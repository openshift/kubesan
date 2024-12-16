// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"

	"github.com/kubesan/kubesan/api/v1alpha1"
	"github.com/kubesan/kubesan/internal/common/commands"
	"github.com/kubesan/kubesan/internal/common/config"
	kubesanslices "github.com/kubesan/kubesan/internal/common/slices"
)

type ThinPoolLvNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpThinPoolLvNodeReconciler(mgr ctrl.Manager) error {
	r := &ThinPoolLvNodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	// KubeSAN VGs use their own LVM profile to avoid interfering with the
	// system-wide lvm.conf configuration. This profile is hardcoded here and is
	// put in place before creating LVs that get their configuration from the
	// profile.

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ThinPoolLv{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=kubesan.openshift.io,resources=thinpoollvs,verbs=get;list;watch;create;update;patch;delete,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.openshift.io,resources=thinpoollvs/status,verbs=get;update;patch,namespace=kubesan-system

func (r *ThinPoolLvNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("nodeName", config.LocalNodeName)

	log.Info("ThinPoolLvNodeReconciler entered")
	defer log.Info("ThinPoolLvNodeReconciler exited")

	thinPoolLv := &v1alpha1.ThinPoolLv{}
	if err := r.Get(ctx, req.NamespacedName, thinPoolLv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// only run after the cluster controller has created the thin-pool

	if !conditionsv1.IsStatusConditionTrue(thinPoolLv.Status.Conditions, conditionsv1.ConditionAvailable) {
		return ctrl.Result{}, nil
	}

	// TODO rebuild Status from on-disk thin-pool state to achieve fault tolerance (e.g. etcd out of sync with disk)

	log.Info("ThinPoolLv is activated, proceeding with node reconcile()")

	stayActive, err := r.reconcileThinPoolLvActivation(ctx, thinPoolLv)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Thin LV commands like lvcreate(8) and lvremove(8) leave the
	// thin-pool activated. Make sure to deactivate the thin-pool before
	// returning, unless a thin LV has been activated.

	if !stayActive {
		defer func() {
			_, _ = commands.Lvm(
				"lvchange",
				"--devicesfile", thinPoolLv.Spec.VgName,
				"--activate", "n",
				fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinPoolLv.Name),
			)
		}()
	}

	// only continue when this node is the active node and we're not undergoing deletion

	if thinPoolLv.DeletionTimestamp != nil || thinPoolLv.Spec.ActiveOnNode != config.LocalNodeName {
		log.Info("ThinPoolLv is not active on this node or is being deleted")
		return ctrl.Result{}, nil
	}

	log.Info("ThinPoolLv is active on this node, proceeding")

	err = r.reconcileThinLvCreation(ctx, thinPoolLv)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileThinLvActivations(ctx, thinPoolLv)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileThinLvDeletion(ctx, thinPoolLv)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO thin LV expansion

	return ctrl.Result{}, nil
}

// Returns true if the thin-pool should be active
func (r *ThinPoolLvNodeReconciler) reconcileThinPoolLvActivation(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) (bool, error) {
	thinPoolLvShouldBeActive := thinPoolLv.DeletionTimestamp == nil &&
		kubesanslices.Any(thinPoolLv.Spec.ThinLvs, func(spec v1alpha1.ThinLvSpec) bool { return spec.State.Name == v1alpha1.ThinLvSpecStateNameActive })

	if thinPoolLvShouldBeActive {
		if thinPoolLv.Spec.ActiveOnNode == config.LocalNodeName {
			// activate LVM thin pool LV

			_, err := commands.Lvm(
				"lvchange",
				"--devicesfile", thinPoolLv.Spec.VgName,
				"--activate", "ey",
				fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinPoolLv.Name),
			)
			if err != nil {
				return thinPoolLvShouldBeActive, err
			}

			condition := conditionsv1.Condition{
				Type:   v1alpha1.ThinPoolLvConditionActive,
				Status: corev1.ConditionTrue,
			}
			conditionsv1.SetStatusCondition(&thinPoolLv.Status.Conditions, condition)

			thinPoolLv.Status.ActiveOnNode = config.LocalNodeName

			if err := r.statusUpdate(ctx, thinPoolLv); err != nil {
				return thinPoolLvShouldBeActive, err
			}
		}
	} else {
		if thinPoolLv.Status.ActiveOnNode == config.LocalNodeName {
			// Deactivate all LVM thin LVs. The `vgchange
			// --activate n --force` flag could be used on the
			// thin-pool LV instead of deactivating thin LVs
			// individually. However, the `--force` flag would hide
			// issues like thin LVs going out of sync with
			// Status.ThinLvs[] so fail noisily to aid debugging.

			for i := range thinPoolLv.Status.ThinLvs {
				thinLvStatus := &thinPoolLv.Status.ThinLvs[i]

				_, err := commands.Lvm(
					"lvchange",
					"--devicesfile", thinPoolLv.Spec.VgName,
					"--activate", "n",
					fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinLvStatus.Name),
				)
				if err != nil {
					return thinPoolLvShouldBeActive, err
				}
			}

			for i := range thinPoolLv.Status.ThinLvs {
				thinLvStatus := &thinPoolLv.Status.ThinLvs[i]

				thinLvStatus.State = v1alpha1.ThinLvStatusState{
					Name: v1alpha1.ThinLvStatusStateNameInactive,
				}
			}

			// deactivate LVM thin pool LV

			_, err := commands.Lvm(
				"lvchange",
				"--devicesfile", thinPoolLv.Spec.VgName,
				"--activate", "n",
				fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinPoolLv.Name),
			)
			if err != nil {
				return thinPoolLvShouldBeActive, err
			}

			condition := conditionsv1.Condition{
				Type:   v1alpha1.ThinPoolLvConditionActive,
				Status: corev1.ConditionFalse,
			}
			conditionsv1.SetStatusCondition(&thinPoolLv.Status.Conditions, condition)

			thinPoolLv.Status.ActiveOnNode = ""

			if err := r.statusUpdate(ctx, thinPoolLv); err != nil {
				return thinPoolLvShouldBeActive, err
			}
		}
	}

	return thinPoolLvShouldBeActive, nil
}

func (r *ThinPoolLvNodeReconciler) reconcileThinLvDeletion(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	needUpdate := false

	// remove thin LVs from thin-pool that have been marked for removal in Spec.ThinLvs[]

	for i := 0; i < len(thinPoolLv.Spec.ThinLvs); i++ {
		thinLvSpec := &thinPoolLv.Spec.ThinLvs[i]
		if thinLvSpec.State.Name != v1alpha1.ThinLvSpecStateNameRemoved {
			continue
		}

		log := log.FromContext(ctx)
		log.Info("Deleting", "thin LV", thinLvSpec.Name)

		err := r.removeThinLv(ctx, thinPoolLv, thinLvSpec.Name)
		if err != nil {
			return err
		}

		thinLvStatus := thinPoolLv.Status.FindThinLv(thinLvSpec.Name)
		if thinLvStatus == nil {
			// This shouldn't happen since ThinLvStatus is set up
			// during thin LV creation, but just in case...
			thinLvStatus := v1alpha1.ThinLvStatus{
				Name: thinLvSpec.Name,
				State: v1alpha1.ThinLvStatusState{
					Name: v1alpha1.ThinLvStatusStateNameRemoved,
				},
				SizeBytes: thinLvSpec.SizeBytes,
			}

			thinPoolLv.Status.ThinLvs = append(thinPoolLv.Status.ThinLvs, thinLvStatus)
		} else {
			thinLvStatus.State = v1alpha1.ThinLvStatusState{
				Name: v1alpha1.ThinLvStatusStateNameRemoved,
			}
		}

		needUpdate = true
	}

	// drop removed thin LVs from Status.ThinLvs[] that no longer appear in Spec.ThinLvs[]

	newThinLvs := make([]v1alpha1.ThinLvStatus, 0, len(thinPoolLv.Spec.ThinLvs))
	for i := 0; i < len(thinPoolLv.Status.ThinLvs); i++ {
		thinLvStatus := &thinPoolLv.Status.ThinLvs[i]
		if thinLvStatus.State.Name == v1alpha1.ThinLvStatusStateNameRemoved && thinPoolLv.Spec.FindThinLv(thinLvStatus.Name) == nil {
			needUpdate = true // a thin LV was dropped
		} else {
			newThinLvs = append(newThinLvs, *thinLvStatus)
		}
	}
	thinPoolLv.Status.ThinLvs = newThinLvs

	if needUpdate {
		if err := r.statusUpdate(ctx, thinPoolLv); err != nil {
			return err
		}
	}
	return nil
}

func (r *ThinPoolLvNodeReconciler) reconcileThinLvCreation(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	for i := range thinPoolLv.Spec.ThinLvs {
		thinLvSpec := &thinPoolLv.Spec.ThinLvs[i]

		if thinPoolLv.Status.FindThinLv(thinLvSpec.Name) == nil {
			log := log.FromContext(ctx)
			log.Info("Creating", "thin LV", thinLvSpec.Name)

			err := r.createThinLv(ctx, thinPoolLv, thinLvSpec)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ThinPoolLvNodeReconciler) reconcileThinLvActivations(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	// deactivate thin LVs that are active on this node but shouldn't be

	for i := range thinPoolLv.Status.ThinLvs {
		thinLvStatus := &thinPoolLv.Status.ThinLvs[i]
		thinLvSpec := thinPoolLv.Spec.FindThinLv(thinLvStatus.Name)

		shouldBeActive := thinLvSpec != nil && thinLvSpec.State.Name == v1alpha1.ThinLvSpecStateNameActive
		isActiveInStatus := thinLvStatus.State.Name == v1alpha1.ThinLvStatusStateNameActive

		path := fmt.Sprintf("/dev/%s/%s", thinPoolLv.Spec.VgName, thinLvStatus.Name)
		isActuallyActive, err := commands.PathExistsOnHost(path)
		if err != nil {
			return err
		}

		if shouldBeActive && !isActuallyActive {
			// activate LVM thin LV

			_, err = commands.Lvm(
				"lvchange",
				"--devicesfile", thinPoolLv.Spec.VgName,
				"--activate", "ey",
				fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinLvStatus.Name),
			)
			if err != nil {
				return err
			}

			// update status to reflect reality if necessary

			if !isActiveInStatus {
				thinLvStatus.State = v1alpha1.ThinLvStatusState{
					Name: v1alpha1.ThinLvStatusStateNameActive,
					Active: &v1alpha1.ThinLvStatusStateActive{
						Path: path,
					},
				}

				if err := r.statusUpdate(ctx, thinPoolLv); err != nil {
					return err
				}
			}
		} else if !shouldBeActive && isActuallyActive {
			// deactivate LVM thin LV

			_, err = commands.Lvm(
				"lvchange",
				"--devicesfile", thinPoolLv.Spec.VgName,
				"--activate", "n",
				fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinLvStatus.Name),
			)
			if err != nil {
				return err
			}

			// update status to reflect reality if necessary

			if isActiveInStatus {
				thinLvStatus.State = v1alpha1.ThinLvStatusState{
					Name: v1alpha1.ThinLvStatusStateNameInactive,
				}

				if err := r.statusUpdate(ctx, thinPoolLv); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *ThinPoolLvNodeReconciler) createThinLv(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv, thinLvSpec *v1alpha1.ThinLvSpec) error {
	log := log.FromContext(ctx)

	switch thinLvSpec.Contents.ContentsType {
	case v1alpha1.ThinLvContentsTypeEmpty:
		// create empty LVM thin LV
		log.Info("Creating an empty thin LV")

		_, err := commands.LvmLvCreateIdempotent(
			"--devicesfile", thinPoolLv.Spec.VgName,
			"--type", "thin",
			"--name", thinLvSpec.Name,
			"--thinpool", thinPoolLv.Name,
			"--virtualsize", fmt.Sprintf("%db", thinLvSpec.SizeBytes),
			thinPoolLv.Spec.VgName,
		)
		if err != nil {
			return err
		}

		// deactivate LVM thin LV (`--activate n` has no effect on `lvcreate --type thin`)

		_, err = commands.Lvm(
			"lvchange",
			"--devicesfile", thinPoolLv.Spec.VgName,
			"--activate", "n",
			fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinLvSpec.Name),
		)
		if err != nil {
			return err
		}

	case v1alpha1.ThinLvContentsTypeSnapshot:
		if thinLvSpec.Contents.Snapshot == nil {
			log.Info("Missing snapshot contents", "thin LV", thinLvSpec.Name)
			return nil
		}

		log.Info("Creating a snapshot LV")

		sourceLv := thinLvSpec.Contents.Snapshot.SourceThinLvName

		if thinPoolLv.Status.FindThinLv(sourceLv) == nil {
			// source thin LV does not (currently) exist
			return nil
		}

		// create snapshot LVM thin LV

		_, err := commands.LvmLvCreateIdempotent(
			"--devicesfile", thinPoolLv.Spec.VgName,
			"--name", thinLvSpec.Name,
			"--snapshot",
			"--setactivationskip", "n",
			fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, sourceLv),
		)
		if err != nil {
			return err
		}

	default:
		// unknown LVM thin LV contents
		log.Info("Unknown contents", "thin LV", thinLvSpec.Name)
		return nil
	}

	thinLvStatus := v1alpha1.ThinLvStatus{
		Name: thinLvSpec.Name,
		State: v1alpha1.ThinLvStatusState{
			Name: v1alpha1.ThinLvStatusStateNameInactive,
		},
		SizeBytes: thinLvSpec.SizeBytes,
	}

	thinPoolLv.Status.ThinLvs = append(thinPoolLv.Status.ThinLvs, thinLvStatus)

	if err := r.statusUpdate(ctx, thinPoolLv); err != nil {
		return err
	}

	log.Info("Successfully created", "thin LV", thinLvSpec.Name)

	return nil
}

func (r *ThinPoolLvNodeReconciler) removeThinLv(_ context.Context, thinPoolLv *v1alpha1.ThinPoolLv, thinLvName string) error {
	_, err := commands.LvmLvRemoveIdempotent(
		"--devicesfile", thinPoolLv.Spec.VgName,
		fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinLvName),
	)
	return err
}

func (r *ThinPoolLvNodeReconciler) statusUpdate(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	thinPoolLv.Status.ObservedGeneration = thinPoolLv.Generation
	return r.Status().Update(ctx, thinPoolLv)
}
