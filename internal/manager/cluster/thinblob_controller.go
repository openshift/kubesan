// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
)

type ThinBlobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpThinBlobReconciler(mgr ctrl.Manager) error {
	r := &ThinBlobReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ThinBlob{}).
		Owns(&v1alpha1.DeviceSwitch{}).
		Owns(&v1alpha1.ThinPoolLv{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinblobs,verbs=get;list;watch;create;update;patch;delete,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinblobs/status,verbs=get;update;patch,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinblobs/finalizers,verbs=update,namespace=kubesan-system

func (r *ThinBlobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, errors.NewBadRequest("not implemented") // TODO

	// thinBlob := &v1alpha1.ThinBlob{}
	// if err := r.Get(ctx, req.NamespacedName, thinBlob); err != nil {
	// 	return ctrl.Result{}, client.IgnoreNotFound(err)
	// }

	// // check if being deleted

	// if thinBlob.DeletionTimestamp != nil {
	// 	// TODO
	// }

	// // create ThinPoolLv

	// thinPoolLv, err := r.createThinPoolLv(ctx, thinBlob)
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// // configure thin LV

	// thinLv := thinPoolLv.Spec.FindThinLv(thinBlob.Name)
	// if thinLv == nil {
	// 	thinLv = &v1alpha1.ThinLvSpec{
	// 		Name: thinBlob.Name,
	// 	}

	// 	if thinBlob.Spec.Contents.Empty != nil {
	// 	} else if thinBlob.Spec.Contents.Snapshot != nil {
	// 		thinLv.Snapshot = thinBlob.Spec.Contents.Snapshot
	// 	}

	// 	thinPoolLv.Spec.ThinLvs = append(thinPoolLv.Spec.ThinLvs, *thinLv)
	// }

	// if !thinBlob.Status.Ready {

	// 	thinBlob.Status.Ready = true

	// 	if err := r.Status().Update(ctx, thinBlob); err != nil {
	// 		return ctrl.Result{}, err
	// 	}
	// }

	// return ctrl.Result{}, nil
}

// func (r *ThinBlobReconciler) createThinPoolLv(ctx context.Context, thinBlob *v1alpha1.ThinBlob) (*v1alpha1.ThinPoolLv, error) {
// 	if thinBlob.Spec.Contents.Snapshot != nil {
// 		sourceThinBlob := &v1alpha1.ThinBlob{}
// 		if err := r.Get(ctx, client.ObjectKey{Name: thinBlob.Spec.Contents.Snapshot.Name}, sourceThinBlob); err != nil {
// 			return nil, err
// 		}
// 	}

// 	thinPoolLv := &v1alpha1.ThinPoolLv{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: thinBlob.Name,
// 		},
// 		Spec: v1alpha1.ThinPoolLvSpec{
// 			VgName: thinBlob.Spec.VgName,
// 		},
// 	}

// 	if err := controllerutil.SetOwnerReference(thinBlob, thinPoolLv, r.Scheme); err != nil {
// 		return nil, err
// 	}

// 	if err := r.Create(ctx, thinPoolLv); err != nil && !errors.IsAlreadyExists(err) {
// 		return nil, err
// 	}

// 	thinPoolLv := &v1alpha1.ThinPoolLv{}
// 	if err := r.Get(ctx, req.NamespacedName, thinPoolLv); err != nil {
// 		return nil, err
// 	}

// 	return thinPoolLv, nil
// }
