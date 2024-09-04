package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"

	apiv1alpha1 "gitlab.com/kubesan/kubesan/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/internal/common/commands"
	"gitlab.com/kubesan/kubesan/internal/common/config"
)

// ThinPoolReconciler reconciles a ThinPool object
type ThinPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpThinPoolReconciler(mgr ctrl.Manager) error {
	r := &ThinPoolReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.ThinPool{}).
		Complete(r)
}

func (r *ThinPoolReconciler) numVolumesAndSnapshots() int {
	return 0 // TODO implement this using labels
}

func (r *ThinPoolReconciler) reconcileFinalizing(ctx context.Context, thinPool *apiv1alpha1.ThinPool) error {
	_, err := commands.LvmLvRemoveIdempotent(
		"--devicesfile", thinPool.Spec.VgName,
		fmt.Sprintf("%s/%s", thinPool.Spec.VgName, thinPool.Name),
	)
	if err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(thinPool, config.Finalizer)

	return r.Update(ctx, thinPool)
}

// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinPools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinPools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinPools/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ThinPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	thinPool := &apiv1alpha1.ThinPool{}
	if err := r.Get(ctx, req.NamespacedName, thinPool); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// only finalize once there are no more Volumes or Snapshots

	if thinPool.DeletionTimestamp != nil && r.numVolumesAndSnapshots() == 0 {
		return ctrl.Result{}, r.reconcileFinalizing(ctx, thinPool)
	}

	// add finalizer

	if !controllerutil.ContainsFinalizer(thinPool, config.Finalizer) {
		controllerutil.AddFinalizer(thinPool, config.Finalizer)

		if err := r.Update(ctx, thinPool); err != nil {
			return ctrl.Result{}, err
		}
	}

	// create thin-pool if necessary

	if !conditionsv1.IsStatusConditionTrue(thinPool.Status.Conditions, conditionsv1.ConditionAvailable) {
		_, err := commands.LvmLvCreateIdempotent(
			"--devicesfile", thinPool.Spec.VgName,
			"--activate", "n",
			"--type", "thin-pool",
			"--metadataprofile", "kubesan",
			"--name", thinPool.Name,
			"--size", "1g", // TODO: find a reasonable heuristic for this
			thinPool.Spec.VgName,
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		condition := conditionsv1.Condition{
			Type:   conditionsv1.ConditionAvailable,
			Status: corev1.ConditionTrue,
		}
		conditionsv1.SetStatusCondition(&thinPool.Status.Conditions, condition)

		if err := r.Status().Update(ctx, thinPool); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
