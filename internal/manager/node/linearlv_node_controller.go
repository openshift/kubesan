// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/internal/common/commands"
	"gitlab.com/kubesan/kubesan/internal/common/config"
	kubesanslices "gitlab.com/kubesan/kubesan/internal/common/slices"
)

type LinearLvNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpLinearLvNodeReconciler(mgr ctrl.Manager) error {
	r := &LinearLvNodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LinearLv{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=linearlvs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubesan.gitlab.io,resources=linearlvs/status,verbs=get;update;patch

func (r *LinearLvNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// TODO: Avoid running while the cluster-wide LinearLv controller or another instance of the node-local LinearLv
	// controller is reconciling the same LinearLv, OR make sure that there are no races.

	linearLv := &v1alpha1.LinearLv{}
	if err := r.Get(ctx, req.NamespacedName, linearLv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if already created

	if !linearLv.Status.Created {
		return ctrl.Result{}, nil
	}

	shouldBeActive := linearLv.DeletionTimestamp == nil && slices.Contains(linearLv.Spec.ActivateOnNodes, config.LocalNodeName)
	isActiveInStatus := slices.Contains(linearLv.Status.ActiveOnNodes, config.LocalNodeName)

	path := fmt.Sprintf("/dev/%s/%s", linearLv.Spec.VgName, linearLv.Name)
	isActuallyActive, err := commands.PathExistsOnHost(path)
	if err != nil {
		return ctrl.Result{}, err
	}

	if shouldBeActive && !isActuallyActive {
		// activate LVM LV on local node

		_, err := commands.Lvm(
			"lvchange",
			"--devicesfile", linearLv.Spec.VgName,
			"--activate", "sy",
			fmt.Sprintf("%s/%s", linearLv.Spec.VgName, linearLv.Name),
		)
		if err != nil {
			return ctrl.Result{}, err
		}
		isActuallyActive = true
	} else if !shouldBeActive && isActuallyActive {
		// deactivate LVM LV from local node

		_, err := commands.Lvm(
			"lvchange",
			"--devicesfile", linearLv.Spec.VgName,
			"--activate", "n",
			fmt.Sprintf("%s/%s", linearLv.Spec.VgName, linearLv.Name),
		)
		if err != nil {
			return ctrl.Result{}, err
		}
		isActuallyActive = false
	}

	// update status to reflect reality if necessary

	if !isActiveInStatus && isActuallyActive {
		linearLv.Status.ActiveOnNodes = append(linearLv.Status.ActiveOnNodes, config.LocalNodeName)
	} else if isActiveInStatus && !isActuallyActive {
		linearLv.Status.ActiveOnNodes = kubesanslices.RemoveAll(linearLv.Status.ActiveOnNodes, config.LocalNodeName)
	} else {
		return ctrl.Result{}, nil // done, no need to update Status
	}

	err = r.Status().Update(ctx, linearLv)
	return ctrl.Result{}, err
}
