// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/internal/common/commands"
	"gitlab.com/kubesan/kubesan/internal/common/config"
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

func (r *VolumeReconciler) reconcileFatNotDeleting(ctx context.Context, volume *v1alpha1.Volume) error {
	// add finalizer

	if !controllerutil.ContainsFinalizer(volume, config.Finalizer) {
		controllerutil.AddFinalizer(volume, config.Finalizer)

		if err := r.Status().Update(ctx, volume); err != nil {
			return err
		}
	}

	// create LVM LV if necessary

	if !volume.Status.Created {
		_, err := commands.LvmLvCreateIdempotent(
			"--devicesfile", volume.Spec.VgName,
			"--activate", "n",
			"--type", "linear",
			"--metadataprofile", "kubesan",
			"--name", volume.Name,
			"--size", fmt.Sprintf("%db", volume.Spec.SizeBytes),
			volume.Spec.VgName,
		)
		if err != nil {
			return err
		}

		volume.Status.Created = true
		volume.Status.SizeBytes = volume.Spec.SizeBytes // TODO report actual size?

		path := fmt.Sprintf("/dev/%s/%s", volume.Spec.VgName, volume.Name)
		volume.Status.Path = &path

		if err := r.Status().Update(ctx, volume); err != nil {
			return err
		}
	}

	// expand LVM LV if necessary

	if volume.Status.SizeBytes < volume.Spec.SizeBytes {
		// TODO: make sure this is idempotent
		_, err := commands.Lvm(
			"lvextend",
			"--devicesfile", volume.Spec.VgName,
			"--size", fmt.Sprintf("%db", volume.Spec.SizeBytes),
			fmt.Sprintf("%s/%s", volume.Spec.VgName, volume.Name),
		)
		if err != nil {
			return err
		}

		volume.Status.SizeBytes = volume.Spec.SizeBytes

		if err := r.Status().Update(ctx, volume); err != nil {
			return err
		}
	}

	return nil
}

func (r *VolumeReconciler) reconcileFatDeleting(ctx context.Context, volume *v1alpha1.Volume) error {
	if len(volume.Status.AttachedToNodes) == 0 {
		_, err := commands.LvmLvRemoveIdempotent(
			"--devicesfile", volume.Spec.VgName,
			fmt.Sprintf("%s/%s", volume.Spec.VgName, volume.Name),
		)
		if err != nil {
			return err
		}

		controllerutil.RemoveFinalizer(volume, config.Finalizer)

		if err := r.Status().Update(ctx, volume); err != nil {
			return err
		}
	}

	return nil
}

func (r *VolumeReconciler) reconcileFat(ctx context.Context, volume *v1alpha1.Volume) error {
	if volume.DeletionTimestamp != nil {
		return r.reconcileFatDeleting(ctx, volume) // TODO
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

	return r.reconcileFatNotDeleting(ctx, volume)
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
