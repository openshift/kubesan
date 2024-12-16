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

	"github.com/kubesan/kubesan/api/v1alpha1"
	"github.com/kubesan/kubesan/internal/common/config"
	"github.com/kubesan/kubesan/internal/common/nbd"
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
		Complete(r)
}

// +kubebuilder:rbac:groups=kubesan.openshift.io,resources=nbdexports,verbs=get;list;watch;create;update;patch;delete,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.openshift.io,resources=nbdexports/status,verbs=get;update;patch,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.openshift.io,resources=nbdexports/finalizers,verbs=update,namespace=kubesan-system

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

	if export.Spec.Path == "" {
		condition := conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  corev1.ConditionFalse,
			Reason:  "Stopping",
			Message: "server stop requested, waiting for clients to disconnect",
		}
		conditionsv1.SetStatusCondition(&export.Status.Conditions, condition)
		if err := r.statusUpdate(ctx, export); err != nil {
			return ctrl.Result{}, err
		}
	}

	serverId := &nbd.ServerId{
		Node:   config.LocalNodeName,
		Export: export.Spec.Export,
	}

	if export.Status.Uri == "" {
		log.Info("Starting NBD export")

		uri, err := nbd.StartServer(ctx, serverId, export.Spec.Path)
		if err != nil {
			return ctrl.Result{}, err
		}
		export.Status.Uri = uri
		condition := conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  corev1.ConditionTrue,
			Reason:  "Ready",
			Message: "NBD Export is ready",
		}
		conditionsv1.SetStatusCondition(&export.Status.Conditions, condition)
		if err = r.statusUpdate(ctx, export); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("Checking NBD export status")
	if err := nbd.CheckServerHealth(ctx, serverId); err != nil {
		condition := conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  corev1.ConditionFalse,
			Reason:  "DeviceError",
			Message: "unexpected NBD server error",
		}
		conditionsv1.SetStatusCondition(&export.Status.Conditions, condition)
		if err := r.statusUpdate(ctx, export); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *NbdExportNodeReconciler) reconcileDeleting(ctx context.Context, export *v1alpha1.NbdExport) error {
	// Mark the export unavailable, so no new clients attach
	if !nbd.ExportDegraded(export) {
		condition := conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  corev1.ConditionFalse,
			Reason:  "Deleting",
			Message: "deletion requested, waiting for clients to disconnect",
		}
		conditionsv1.SetStatusCondition(&export.Status.Conditions, condition)
		if err := r.statusUpdate(ctx, export); err != nil {
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
	if err := nbd.StopServer(ctx, serverId); err != nil {
		return err
	}

	// Now the CR can be deleted
	controllerutil.RemoveFinalizer(export, config.Finalizer)
	return r.Update(ctx, export)
}

func (r *NbdExportNodeReconciler) statusUpdate(ctx context.Context, export *v1alpha1.NbdExport) error {
	export.Status.ObservedGeneration = export.Generation
	return r.Status().Update(ctx, export)
}
