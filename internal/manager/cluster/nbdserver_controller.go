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

type NbdServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpNbdServerReconciler(mgr ctrl.Manager) error {
	r := &NbdServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NbdServer{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=nbdservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=nbdservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=nbdservers/finalizers,verbs=update

func (r *NbdServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, errors.NewBadRequest("not implemented") // TODO
}
