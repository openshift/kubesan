// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	kubesanslices "gitlab.com/kubesan/kubesan/internal/common/slices"
)

type FatBlobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpFatBlobReconciler(mgr ctrl.Manager) error {
	r := &FatBlobReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.FatBlob{}).
		Owns(&v1alpha1.LinearLv{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=fatblobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=fatblobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=fatblobs/finalizers,verbs=update

//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=linearlvs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=linearlvs/status,verbs=get

func (r *FatBlobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	fatBlob := &v1alpha1.FatBlob{}
	if err := r.Get(ctx, req.NamespacedName, fatBlob); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if fatBlob.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	linearLv := &v1alpha1.LinearLv{}
	err := r.Get(ctx, client.ObjectKeyFromObject(fatBlob), linearLv)

	if err == nil {
		err = r.updateLinearLvSpec(ctx, fatBlob, linearLv)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.updateFatBlobStatus(ctx, fatBlob, linearLv)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	} else if errors.IsNotFound(err) {
		err = r.createLinearLv(ctx, fatBlob)
		return ctrl.Result{}, err
	} else {
		return ctrl.Result{}, err
	}
}

func (r *FatBlobReconciler) createLinearLv(ctx context.Context, fatBlob *v1alpha1.FatBlob) error {
	linearLv := &v1alpha1.LinearLv{
		ObjectMeta: metav1.ObjectMeta{
			Name: fatBlob.Name,
		},
		Spec: v1alpha1.LinearLvSpec{
			VgName:          fatBlob.Spec.VgName,
			ReadOnly:        fatBlob.Spec.ReadOnly,
			SizeBytes:       fatBlob.Spec.SizeBytes,
			ActivateOnNodes: fatBlob.Spec.AttachToNodes,
		},
	}

	if err := controllerutil.SetControllerReference(fatBlob, linearLv, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, linearLv); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (r *FatBlobReconciler) updateLinearLvSpec(ctx context.Context, fatBlob *v1alpha1.FatBlob, linearLv *v1alpha1.LinearLv) error {
	var updated bool

	if linearLv.Spec.SizeBytes != fatBlob.Spec.SizeBytes {
		linearLv.Spec.SizeBytes = fatBlob.Spec.SizeBytes
		updated = true
	}

	if !kubesanslices.SetsEqual(linearLv.Spec.ActivateOnNodes, fatBlob.Spec.AttachToNodes) {
		linearLv.Spec.ActivateOnNodes = kubesanslices.Deduplicate(fatBlob.Spec.AttachToNodes)
		updated = true
	}

	if updated {
		if err := r.Update(ctx, linearLv); err != nil {
			return err
		}
	}

	return nil
}

func (r *FatBlobReconciler) updateFatBlobStatus(ctx context.Context, fatBlob *v1alpha1.FatBlob, linearLv *v1alpha1.LinearLv) error {
	var updated bool

	if fatBlob.Status.Created != linearLv.Status.Created {
		fatBlob.Status.Created = linearLv.Status.Created
		updated = true
	}

	if fatBlob.Status.SizeBytes != linearLv.Status.SizeBytes {
		fatBlob.Status.SizeBytes = linearLv.Status.SizeBytes
		updated = true
	}

	if !kubesanslices.SetsEqual(fatBlob.Status.AttachedToNodes, linearLv.Status.ActiveOnNodes) {
		fatBlob.Status.AttachedToNodes = kubesanslices.Deduplicate(linearLv.Status.ActiveOnNodes)
		updated = true
	}

	if fatBlob.Status.GetPath() != linearLv.Status.GetPath() {
		fatBlob.Status.Path = linearLv.Status.Path
		updated = true
	}

	if updated {
		if err := r.Status().Update(ctx, fatBlob); err != nil {
			return err
		}
	}

	return nil
}
