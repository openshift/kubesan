// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/internal/common/commands"
	"gitlab.com/kubesan/kubesan/internal/common/config"
)

type ThinPoolLvReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpThinPoolLvReconciler(mgr ctrl.Manager) error {
	r := &ThinPoolLvReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ThinPoolLv{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinpoollvs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinpoollvs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinpoollvs/finalizers,verbs=update

func (r *ThinPoolLvReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	thinPoolLv := &v1alpha1.ThinPoolLv{}
	if err := r.Get(ctx, req.NamespacedName, thinPoolLv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var err error

	if thinPoolLv.DeletionTimestamp == nil {
		err = r.reconcileNotDeleting(ctx, thinPoolLv)
	} else {
		err = r.reconcileDeleting(ctx, thinPoolLv)
	}

	return ctrl.Result{}, err
}

func (r *ThinPoolLvReconciler) reconcileNotDeleting(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	// add finalizer

	if !controllerutil.ContainsFinalizer(thinPoolLv, config.Finalizer) {
		controllerutil.AddFinalizer(thinPoolLv, config.Finalizer)

		if err := r.Status().Update(ctx, thinPoolLv); err != nil {
			return err
		}
	}

	// create LVM thin pool LV

	if !thinPoolLv.Status.Created {
		err := r.createThinPoolLv(ctx, thinPoolLv)
		if err != nil {
			return err
		}
	}

	// create LVM thin LVs

	for i := range thinPoolLv.Spec.ThinLvs {
		thinLvSpec := &thinPoolLv.Spec.ThinLvs[i]

		if thinPoolLv.Status.FindThinLv(thinLvSpec.Name) == nil {
			err := r.createThinLv(ctx, thinPoolLv, thinLvSpec)
			if err != nil {
				return err
			}
		}
	}

	// remove LVM thin LVs

	for i := 0; i < len(thinPoolLv.Status.ThinLvs); i++ {
		thinLvStatus := &thinPoolLv.Status.ThinLvs[i]

		if thinPoolLv.Spec.FindThinLv(thinLvStatus.Name) == nil && thinLvStatus.State.Inactive != nil {
			err := r.removeThinLv(ctx, thinPoolLv, thinLvStatus.Name)
			if err != nil {
				return err
			}

			thinPoolLv.Status.ThinLvs = slices.Delete(thinPoolLv.Status.ThinLvs, i, i+1)
			i--
		}
	}

	return nil
}

func (r *ThinPoolLvReconciler) reconcileDeleting(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	// remove LVM thin LVs

	for i := range thinPoolLv.Status.ThinLvs {
		thinLv := &thinPoolLv.Status.ThinLvs[i]

		if thinLv.State.Inactive != nil {
			err := r.removeThinLv(ctx, thinPoolLv, thinLv.Name)
			if err != nil {
				return err
			}

			thinPoolLv.Status.ThinLvs = slices.Delete(thinPoolLv.Status.ThinLvs, i, i+1)
		}
	}

	// remove LVM thin pool LV if there are no LVM thin LVs left

	if len(thinPoolLv.Status.ThinLvs) == 0 {
		err := r.removeThinPoolLv(ctx, thinPoolLv)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ThinPoolLvReconciler) createThinPoolLv(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	_, err := commands.LvmLvCreateIdempotent(
		"--devicesfile", thinPoolLv.Spec.VgName,
		"--activate", "n",
		"--type", "thin-pool",
		"--metadataprofile", "kubesan",
		"--name", thinPoolLv.Name,
		"--size", "1g", // TODO: find a reasonable heuristic for this
		thinPoolLv.Spec.VgName,
	)
	if err != nil {
		return err
	}

	thinPoolLv.Status.Created = true

	if err := r.Status().Update(ctx, thinPoolLv); err != nil {
		return err
	}

	return nil
}

func (r *ThinPoolLvReconciler) removeThinPoolLv(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	_, err := commands.LvmLvRemoveIdempotent(
		"--devicesfile", thinPoolLv.Spec.VgName,
		fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinPoolLv.Name),
	)
	if err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(thinPoolLv, config.Finalizer)

	if err := r.Status().Update(ctx, thinPoolLv); err != nil {
		return err
	}

	return nil
}

func (r *ThinPoolLvReconciler) createThinLv(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv, thinLvSpec *v1alpha1.ThinLvSpec) error {
	switch {
	case thinLvSpec.Contents.Empty != nil:
		// create empty LVM thin LV

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

	case thinLvSpec.Contents.Snapshot != nil:
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
		return nil
	}

	thinLvStatus := v1alpha1.ThinLvStatus{
		Name: thinLvSpec.Name,
		State: v1alpha1.ThinLvState{
			Inactive: &v1alpha1.ThinLvStateInactive{},
		},
		SizeBytes: thinLvSpec.SizeBytes,
	}

	thinPoolLv.Status.ThinLvs = append(thinPoolLv.Status.ThinLvs, thinLvStatus)

	if err := r.Status().Update(ctx, thinPoolLv); err != nil {
		return err
	}

	return nil
}

func (r *ThinPoolLvReconciler) removeThinLv(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv, thinLvName string) error {
	_, err := commands.LvmLvRemoveIdempotent(
		"--devicesfile", thinPoolLv.Spec.VgName,
		fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinLvName),
	)
	if err != nil {
		return err
	}

	if err := r.Status().Update(ctx, thinPoolLv); err != nil {
		return err
	}

	return nil
}
