// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"

	//	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/internal/common/commands"
	"gitlab.com/kubesan/kubesan/internal/common/config"
	kubesanslices "gitlab.com/kubesan/kubesan/internal/common/slices"
)

type ThinPoolLvNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpThinPoolLvNodeReconciler(mgr ctrl.Manager) error {
	r := &ThinPoolLvNodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	// KubeSAN VGs use their own LVM profile to avoid interfering with the
	// system-wide lvm.conf configuration. This profile is hardcoded here and is
	// put in place before creating LVs that get their configuration from the
	// profile.

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ThinPoolLv{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinpoollvs,verbs=get;list;watch;create;update;patch;delete,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=thinpoollvs/status,verbs=get;update;patch,namespace=kubesan-system

func (r *ThinPoolLvNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// TODO: Avoid running while the cluster-wide ThinPoolLv controller or another instance of the node-local
	// ThinPoolLv controller is reconciling the same ThinPoolLv, OR make sure that there are no races.

	thinPoolLv := &v1alpha1.ThinPoolLv{}
	if err := r.Get(ctx, req.NamespacedName, thinPoolLv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !conditionsv1.IsStatusConditionTrue(thinPoolLv.Status.Conditions, conditionsv1.ConditionAvailable) {
		return ctrl.Result{}, nil
	}

	err := r.reconcileThinPoolLvActivation(ctx, thinPoolLv)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileThinLvActivations(ctx, thinPoolLv)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ThinPoolLvNodeReconciler) reconcileThinPoolLvActivation(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	thinPoolLvShouldBeActive := thinPoolLv.DeletionTimestamp == nil &&
		kubesanslices.Any(thinPoolLv.Spec.ThinLvs, func(spec v1alpha1.ThinLvSpec) bool { return spec.Activate })

	if thinPoolLvShouldBeActive {
		if thinPoolLv.Spec.ActiveOnNode == config.LocalNodeName {
			if thinPoolLv.Status.ActiveOnNode == "" {
				// activate LVM thin pool LV

				_, err := commands.Lvm(
					"lvchange",
					"--devicesfile", thinPoolLv.Spec.VgName,
					"--activate", "ey",
					fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinPoolLv.Name),
				)
				if err != nil {
					return err
				}

				thinPoolLv.Status.ActiveOnNode = config.LocalNodeName

				if err := r.Status().Update(ctx, thinPoolLv); err != nil {
					return err
				}
			} else if thinPoolLv.Status.ActiveOnNode != config.LocalNodeName {
				// TODO: idempotently (attempt to) activate LVM thin pool LV

				// NOTE: if trying to figure out if thin pool is actually really already activated on another
				// node, simply always try to first activate it on the local node, and if it fails with the
				// right message, we conclude that it is active on another node; otherwise we have "stolen" the
				// activation from a failed node and have to update the ThinPoolLv status

				failedBecauseAlreadyActiveOnAnotherNode := false // TODO: implement
				managedToActivateOnLocalNode := true             // TODO: implement

				if failedBecauseAlreadyActiveOnAnotherNode {
					// all is well
				} else if managedToActivateOnLocalNode {
					// we though the thin pool was already active on another node, but that
					// activation was since lost (node power failure, LVM lease renewal failure,
					// etc.); update status to reflect reality

					thinPoolLv.Status.ActiveOnNode = config.LocalNodeName

					for i := range thinPoolLv.Status.ThinLvs {
						thinLvStatus := &thinPoolLv.Status.ThinLvs[i]

						thinLvStatus.State = v1alpha1.ThinLvState{
							Inactive: &v1alpha1.ThinLvStateInactive{},
						}
					}

					if err := r.Status().Update(ctx, thinPoolLv); err != nil {
						return err
					}
				}
			}
		}
	} else {
		if thinPoolLv.Status.ActiveOnNode == config.LocalNodeName {
			// TODO: idempotently deactivate all LVM thin LVs

			// TODO: idempotently deactivate LVM thin pool LV

			for i := range thinPoolLv.Status.ThinLvs {
				thinLvStatus := &thinPoolLv.Status.ThinLvs[i]

				thinLvStatus.State = v1alpha1.ThinLvState{
					Inactive: &v1alpha1.ThinLvStateInactive{},
				}
			}

			thinPoolLv.Status.ActiveOnNode = ""

			if err := r.Status().Update(ctx, thinPoolLv); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ThinPoolLvNodeReconciler) reconcileThinLvActivations(ctx context.Context, thinPoolLv *v1alpha1.ThinPoolLv) error {
	if thinPoolLv.Status.ActiveOnNode != config.LocalNodeName {
		return nil
	}

	// deactivate thin LVs that are active on this node but shouldn't be

	for i := range thinPoolLv.Status.ThinLvs {
		thinLvStatus := &thinPoolLv.Status.ThinLvs[i]
		thinLvSpec := thinPoolLv.Spec.FindThinLv(thinLvStatus.Name)

		shouldBeActive := thinPoolLv.DeletionTimestamp == nil && thinLvSpec != nil && thinLvSpec.Activate
		isActiveInStatus := thinLvStatus.State.Active != nil

		path := fmt.Sprintf("/dev/%s/%s", thinPoolLv.Spec.VgName, thinLvStatus.Name)
		isActuallyActive, err := commands.PathExistsOnHost(path)
		if err != nil {
			return err
		}

		if shouldBeActive && !isActuallyActive {
			// TODO: idempotently deactivate LVM thin LV

			// update status to reflect reality if necessary

			if !isActiveInStatus {
				thinLvStatus.State = v1alpha1.ThinLvState{
					Active: &v1alpha1.ThinLvStateActive{
						Path: path,
					},
				}

				if err := r.Status().Update(ctx, thinPoolLv); err != nil {
					return err
				}
			}
		} else if !shouldBeActive && isActuallyActive {
			// deactivate LVM thin LV

			_, err = commands.Lvm(
				"lvchange",
				"--devicesfile", thinPoolLv.Spec.VgName,
				"--activate", "n",
				fmt.Sprintf("%s/%s", thinPoolLv.Spec.VgName, thinLvStatus.Name),
			)
			if err != nil {
				return err
			}

			// update status to reflect reality if necessary

			if isActiveInStatus {
				thinLvStatus.State = v1alpha1.ThinLvState{
					Inactive: &v1alpha1.ThinLvStateInactive{},
				}

				if err := r.Status().Update(ctx, thinPoolLv); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
