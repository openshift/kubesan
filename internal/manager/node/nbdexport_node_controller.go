// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/internal/common/config"
	"gitlab.com/kubesan/kubesan/internal/common/nbd"
	"gitlab.com/kubesan/kubesan/internal/manager/common/util"
)

type NbdExportNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpNbdExportNodeReconciler(mgr ctrl.Manager) error {
	r := &NbdExportNodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NbdExport{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=nbdexports,verbs=get;list;watch;create;update;patch;delete,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=nbdexports/status,verbs=get;update;patch,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=nbdexports/finalizers,verbs=update,namespace=kubesan-system
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete,namespace=kubesan-system
// +kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update,namespace=kubesan-system

func (r *NbdExportNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("node", config.LocalNodeName)

	log.Info("NbdExportNodeReconciler entered")
	defer log.Info("NbdExportNodeReconciler exited")

	export := &v1alpha1.NbdExport{}
	if err := r.Get(ctx, req.NamespacedName, export); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if export.Spec.Host != config.LocalNodeName {
		return ctrl.Result{}, nil
	}

	if export.DeletionTimestamp != nil {
		log.Info("Attempting deletion")
		err := r.reconcileDeleting(ctx, export)
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(export, config.Finalizer) {
		controllerutil.AddFinalizer(export, config.Finalizer)

		if err := r.Update(ctx, export); err != nil {
			return ctrl.Result{}, err
		}
	}

	serverId := &nbd.ServerId{
		Node:   config.LocalNodeName,
		Export: export.Spec.Export,
	}

	if export.Status.Uri == "" {
		log.Info("Starting export pod")

		uri, err := nbd.StartServer(ctx, export, r.Scheme, r.Client, serverId, export.Spec.Path)
		if err != nil {
			if _, ok := err.(*util.WatchPending); ok {
				log.Info("StartServer waiting for Pod")
				return ctrl.Result{}, nil // will retry after Pod changes status
			}
			return ctrl.Result{}, err
		}
		export.Status.Uri = uri
		condition := conditionsv1.Condition{
			Type:   conditionsv1.ConditionAvailable,
			Status: corev1.ConditionTrue,
		}
		conditionsv1.SetStatusCondition(&export.Status.Conditions, condition)
		if err = r.Status().Update(ctx, export); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("Checking export pod status")
	if err := nbd.CheckServerHealth(ctx, r.Client, serverId); err != nil {
		condition := conditionsv1.Condition{
			Type:   conditionsv1.ConditionAvailable,
			Status: corev1.ConditionFalse,
		}
		conditionsv1.SetStatusCondition(&export.Status.Conditions, condition)
		if err := r.Status().Update(ctx, export); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *NbdExportNodeReconciler) reconcileDeleting(ctx context.Context, export *v1alpha1.NbdExport) error {
	// Mark the export unavailable, so no new clients attach
	if !conditionsv1.IsStatusConditionFalse(export.Status.Conditions, conditionsv1.ConditionAvailable) {
		condition := conditionsv1.Condition{
			Type:   conditionsv1.ConditionAvailable,
			Status: corev1.ConditionFalse,
		}
		conditionsv1.SetStatusCondition(&export.Status.Conditions, condition)
		if err := r.Status().Update(ctx, export); err != nil {
			return err
		}
	}

	// Wait for all existing clients to detach
	if len(export.Spec.Clients) > 0 {
		return nil // wait until no longer attached
	}

	serverId := &nbd.ServerId{
		Node:   config.LocalNodeName,
		Export: export.Spec.Export,
	}
	if err := nbd.StopServer(ctx, r.Client, serverId); err != nil {
		if _, ok := err.(*util.WatchPending); ok {
			return nil // will retry after Pod changes status
		}
		return err
	}

	// Now the CR can be deleted
	controllerutil.RemoveFinalizer(export, config.Finalizer)
	return r.Update(ctx, export)
}
