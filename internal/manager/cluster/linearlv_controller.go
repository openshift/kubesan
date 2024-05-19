// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/internal/common/commands"
	"gitlab.com/kubesan/kubesan/internal/common/config"
)

type LinearLvReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpLinearLvReconciler(mgr ctrl.Manager) error {
	r := &LinearLvReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LinearLv{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=linearlvs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=linearlvs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=linearlvs/finalizers,verbs=update

func (r *LinearLvReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	linearLv := &v1alpha1.LinearLv{}
	if err := r.Get(ctx, req.NamespacedName, linearLv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var err error

	if linearLv.DeletionTimestamp == nil {
		err = r.reconcileNotDeleting(ctx, linearLv)
	} else {
		err = r.reconcileDeleting(ctx, linearLv)
	}

	return ctrl.Result{}, err
}

func (r *LinearLvReconciler) reconcileNotDeleting(ctx context.Context, linearLv *v1alpha1.LinearLv) error {
	// add finalizer

	if !controllerutil.ContainsFinalizer(linearLv, config.Finalizer) {
		controllerutil.AddFinalizer(linearLv, config.Finalizer)

		if err := r.Status().Update(ctx, linearLv); err != nil {
			return err
		}
	}

	// create LVM LV if necessary

	if !linearLv.Status.Created {
		_, err := commands.LvmLvCreateIdempotent(
			"--devicesfile", linearLv.Spec.VgName,
			"--activate", "n",
			"--type", "linear",
			"--metadataprofile", "kubesan",
			"--name", linearLv.Name,
			"--size", fmt.Sprintf("%db", linearLv.Spec.SizeBytes),
			linearLv.Spec.VgName,
		)
		if err != nil {
			return err
		}

		linearLv.Status.Created = true
		linearLv.Status.SizeBytes = linearLv.Spec.SizeBytes

		path := fmt.Sprintf("/dev/%s/%s", linearLv.Spec.VgName, linearLv.Name)
		linearLv.Status.Path = &path

		if err := r.Status().Update(ctx, linearLv); err != nil {
			return err
		}
	}

	// expand LVM LV if necessary

	if linearLv.Status.SizeBytes < linearLv.Spec.SizeBytes {
		// TODO: make sure this is idempotent
		_, err := commands.Lvm(
			"lvextend",
			"--devicesfile", linearLv.Spec.VgName,
			"--size", fmt.Sprintf("%db", linearLv.Spec.SizeBytes),
			fmt.Sprintf("%s/%s", linearLv.Spec.VgName, linearLv.Name),
		)
		if err != nil {
			return err
		}

		linearLv.Status.SizeBytes = linearLv.Spec.SizeBytes

		if err := r.Status().Update(ctx, linearLv); err != nil {
			return err
		}
	}

	return nil
}

func (r *LinearLvReconciler) reconcileDeleting(ctx context.Context, linearLv *v1alpha1.LinearLv) error {
	if len(linearLv.Status.ActiveOnNodes) == 0 {
		_, err := commands.LvmLvRemoveIdempotent(
			"--devicesfile", linearLv.Spec.VgName,
			fmt.Sprintf("%s/%s", linearLv.Spec.VgName, linearLv.Name),
		)
		if err != nil {
			return err
		}

		controllerutil.RemoveFinalizer(linearLv, config.Finalizer)

		if err := r.Status().Update(ctx, linearLv); err != nil {
			return err
		}
	}

	return nil
}
