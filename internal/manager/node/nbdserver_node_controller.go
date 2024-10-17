// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
)

type NbdServerNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpNbdServerNodeReconciler(mgr ctrl.Manager) error {
	r := &NbdServerNodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NbdServer{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=nbdservers,verbs=get;list;watch;create;update;patch;delete,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=nbdservers/status,verbs=get;update;patch,namespace=kubesan-system

func (r *NbdServerNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, errors.NewBadRequest("not implemented") // TODO
}
