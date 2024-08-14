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

type VolumeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpVolumeReconciler(mgr ctrl.Manager) error {
	r := &VolumeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Volume{}).
		Owns(&v1alpha1.FatBlob{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=volumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=volumes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=volumes/finalizers,verbs=update

func (r *VolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// log := log.FromContext(ctx)

	volume := &v1alpha1.Volume{}
	if err := r.Get(ctx, req.NamespacedName, volume); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var err error

	switch volume.Spec.Mode {
	case v1alpha1.VolumeModeThin:
		err = r.reconcileThin(ctx, volume)
	case v1alpha1.VolumeModeFat:
		err = r.reconcileFat(ctx, volume)
	default:
		err = errors.NewBadRequest("invalid volume mode")
	}

	return ctrl.Result{}, err
}

func (r *VolumeReconciler) reconcileThin(ctx context.Context, volume *v1alpha1.Volume) error {
	return errors.NewBadRequest("not implemented") // TODO
}

func (r *VolumeReconciler) reconcileFat(ctx context.Context, volume *v1alpha1.Volume) error {
	if volume.DeletionTimestamp != nil {
		return nil
	}

	if kubesanslices.CountNonNil(
		volume.Spec.Type.Block,
		volume.Spec.Type.Filesystem,
	) != 1 {
		return errors.NewBadRequest("invalid volume type")
	}

	switch {
	case volume.Spec.Type.Block != nil:
		// nothing to do

	case volume.Spec.Type.Filesystem != nil:
		return errors.NewBadRequest("not implemented") // TODO
	}

	if kubesanslices.CountNonNil(
		volume.Spec.Contents.Empty,
		volume.Spec.Contents.CloneVolume,
		volume.Spec.Contents.CloneSnapshot,
	) != 1 {
		return errors.NewBadRequest("invalid volume contents")
	}

	switch {
	case volume.Spec.Contents.Empty != nil:
		// nothing to do

	case volume.Spec.Contents.CloneVolume != nil:
		return errors.NewBadRequest("cloning volumes is not supported for fat volumes")

	case volume.Spec.Contents.CloneSnapshot != nil:
		return errors.NewBadRequest("cloning snapshots is not supported for fat volumes")
	}

	// create FatBlob or update its spec

	fatBlob := &v1alpha1.FatBlob{}
	err := r.Get(ctx, client.ObjectKeyFromObject(volume), fatBlob)

	if err == nil {
		err = r.updateFatBlobSpec(ctx, volume, fatBlob)
		if err != nil {
			return err
		}

		err = r.updateVolumeStatus(ctx, volume, fatBlob)
		if err != nil {
			return err
		}

		return nil
	} else if errors.IsNotFound(err) {
		return r.createFatBlob(ctx, volume)
	} else {
		return err
	}
}

func (r *VolumeReconciler) createFatBlob(ctx context.Context, volume *v1alpha1.Volume) error {
	fatBlob := &v1alpha1.FatBlob{
		ObjectMeta: metav1.ObjectMeta{
			Name: volume.Name,
		},
		Spec: v1alpha1.FatBlobSpec{
			VgName:        volume.Spec.VgName,
			ReadOnly:      volume.Spec.ReadOnly(),
			SizeBytes:     volume.Spec.SizeBytes,
			AttachToNodes: kubesanslices.Deduplicate(volume.Spec.AttachToNodes),
		},
	}

	if err := controllerutil.SetControllerReference(volume, fatBlob, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, fatBlob); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (r *VolumeReconciler) updateFatBlobSpec(ctx context.Context, volume *v1alpha1.Volume, fatBlob *v1alpha1.FatBlob) error {
	updated := false

	if fatBlob.Spec.SizeBytes != volume.Spec.SizeBytes {
		fatBlob.Spec.SizeBytes = volume.Spec.SizeBytes
		updated = true
	}

	if !kubesanslices.SetsEqual(fatBlob.Spec.AttachToNodes, volume.Spec.AttachToNodes) {
		fatBlob.Spec.AttachToNodes = kubesanslices.Deduplicate(volume.Spec.AttachToNodes)
		updated = true
	}

	if updated {
		if err := r.Update(ctx, fatBlob); err != nil {
			return err
		}
	}

	return nil
}

func (r *VolumeReconciler) updateVolumeStatus(ctx context.Context, volume *v1alpha1.Volume, fatBlob *v1alpha1.FatBlob) error {
	updated := false

	if volume.Status.Created != fatBlob.Status.Created {
		volume.Status.Created = true
		updated = true
	}

	if volume.Status.SizeBytes != fatBlob.Status.SizeBytes {
		volume.Status.SizeBytes = fatBlob.Status.SizeBytes
		updated = true
	}

	if !kubesanslices.SetsEqual(volume.Status.AttachedToNodes, fatBlob.Status.AttachedToNodes) {
		volume.Status.AttachedToNodes = kubesanslices.Deduplicate(fatBlob.Status.AttachedToNodes)
		updated = true
	}

	if volume.Status.GetPath() != fatBlob.Status.GetPath() {
		volume.Status.Path = fatBlob.Status.Path
		updated = true
	}

	if updated {
		if err := r.Client.Status().Update(ctx, volume); err != nil {
			return err
		}
	}

	return nil
}

// func createOrUpdate[T client.Object](ctx context.Context, c client.Client, emptyObj T, update func(obj T) error) (T, error) {
// 	// try creating object

// 	obj := emptyObj.DeepCopyObject().(T)
// 	if err := update(obj); err != nil {
// 		return emptyObj, err
// 	}

// 	if err := c.Create(ctx, obj); err == nil {
// 		return obj, nil
// 	} else if !errors.IsAlreadyExists(err) {
// 		return emptyObj, err
// 	}

// 	// object already exists, update it

// 	obj = emptyObj.DeepCopyObject().(T)

// 	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
// 		return emptyObj, err
// 	}

// 	if err := update(obj); err != nil {
// 		return emptyObj, err
// 	}

// 	if err := c.Update(ctx, obj); err != nil {
// 		return emptyObj, err
// 	}

// 	return obj, nil
// }
