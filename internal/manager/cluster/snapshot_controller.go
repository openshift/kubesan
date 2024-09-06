// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpSnapshotReconciler(mgr ctrl.Manager) error {
	r := &SnapshotReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Snapshot{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=snapshots/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=snapshots/finalizers,verbs=update

func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, errors.NewBadRequest("not implemented") // TODO
}
