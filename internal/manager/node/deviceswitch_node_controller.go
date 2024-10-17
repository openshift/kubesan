// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/internal/common/commands"
	"gitlab.com/kubesan/kubesan/internal/common/config"
)

type DeviceSwitchNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetUpDeviceSwitchNodeReconciler(mgr ctrl.Manager) error {
	r := &DeviceSwitchNodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DeviceSwitch{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=deviceswitches,verbs=get;list;watch;create;update;patch;delete,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=deviceswitches/status,verbs=get;update;patch,namespace=kubesan-system
// +kubebuilder:rbac:groups=kubesan.gitlab.io,resources=deviceswitches/finalizers,verbs=update,namespace=kubesan-system

func (r *DeviceSwitchNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	deviceSwitch := &v1alpha1.DeviceSwitch{}
	if err := r.Get(ctx, req.NamespacedName, deviceSwitch); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	controllerHasJurisdiction := deviceSwitch.Status.Node == &config.LocalNodeName ||
		(deviceSwitch.Status.Node == nil && deviceSwitch.Spec.Node == config.LocalNodeName)

	var err error

	if controllerHasJurisdiction {
		if deviceSwitch.DeletionTimestamp == nil {
			err = r.reconcileNotDeleting(ctx, deviceSwitch)
		} else {
			err = r.reconcileDeleting(ctx, deviceSwitch)
		}
	}

	return ctrl.Result{}, err
}

func (r *DeviceSwitchNodeReconciler) reconcileNotDeleting(ctx context.Context, deviceSwitch *v1alpha1.DeviceSwitch) error {
	// add finalizer

	if !controllerutil.ContainsFinalizer(deviceSwitch, config.Finalizer) {
		controllerutil.AddFinalizer(deviceSwitch, config.Finalizer)

		if err := r.Update(ctx, deviceSwitch); err != nil {
			return err
		}
	}

	// tear down dm-multipath if it is not on the right node

	if deviceSwitch.Spec.Node != config.LocalNodeName {
		err := r.removeDmMultipath(deviceSwitch.Name)
		if err != nil {
			return err
		}

		if deviceSwitch.Status.NbdDevice != nil {
			err := commands.NbdClientDisconnect(*deviceSwitch.Status.NbdDevice)
			if err != nil {
				return err
			}
		}

		deviceSwitch.Status = v1alpha1.DeviceSwitchStatus{}

		if err := r.Status().Update(ctx, deviceSwitch); err != nil {
			return err
		}

		return nil
	}

	// set up dm-multipath if not already set up

	if deviceSwitch.Status.Node == nil {
		path, err := r.createDmMultipath(deviceSwitch.Name, deviceSwitch.Spec.SizeBytes, nil)
		if err != nil {
			return err
		}

		deviceSwitch.Status.Node = &config.LocalNodeName
		deviceSwitch.Status.OutputPath = &path

		sizeBytes := deviceSwitch.Spec.SizeBytes
		deviceSwitch.Status.SizeBytes = &sizeBytes

		if err := r.Status().Update(ctx, deviceSwitch); err != nil {
			return err
		}
	}

	// reconfigure dm-multipath if input URI changed

	newInputURI := deviceSwitch.Spec.InputURI
	newSizeBytes := deviceSwitch.Spec.SizeBytes

	if deviceSwitch.Status.InputURI != newInputURI || *deviceSwitch.Status.SizeBytes != newSizeBytes {
		// disconnect (if necessary)

		if deviceSwitch.Status.InputURI != nil {
			err := r.reconfigureDmMultipath(deviceSwitch.Name, *deviceSwitch.Status.SizeBytes, nil)
			if err != nil {
				return err
			}

			if deviceSwitch.Status.NbdDevice != nil {
				err := commands.NbdClientDisconnect(*deviceSwitch.Status.NbdDevice)
				if err != nil {
					return err
				}
			}

			deviceSwitch.Status.InputURI = nil
			deviceSwitch.Status.NbdDevice = nil

			if err := r.Status().Update(ctx, deviceSwitch); err != nil {
				return err
			}
		}

		// connect (if necessary)

		var newDevPath *string

		if newInputURI != nil && strings.HasPrefix(*newInputURI, "nbd://") {
			nbdDevPath, err := commands.NbdClientConnect(strings.TrimPrefix(*newInputURI, "nbd://"))
			if err != nil {
				return err
			}

			deviceSwitch.Status.NbdDevice = &nbdDevPath

			if err := r.Status().Update(ctx, deviceSwitch); err != nil {
				_ = commands.NbdClientDisconnect(nbdDevPath) // best effort cleanup
				return err
			}

			newDevPath = &nbdDevPath
		} else if newInputURI != nil && strings.HasPrefix(*newInputURI, "file://") {
			path := strings.TrimPrefix(*newInputURI, "file://")
			newDevPath = &path
		}

		if newDevPath != nil {
			err := r.reconfigureDmMultipath(deviceSwitch.Name, newSizeBytes, newDevPath)
			if err != nil {
				return err
			}

			deviceSwitch.Status.SizeBytes = &newSizeBytes
			deviceSwitch.Status.InputURI = newInputURI

			if err := r.Status().Update(ctx, deviceSwitch); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *DeviceSwitchNodeReconciler) reconcileDeleting(ctx context.Context, deviceSwitch *v1alpha1.DeviceSwitch) error {
	err := r.removeDmMultipath(deviceSwitch.Name)
	if err != nil {
		return err
	}

	if deviceSwitch.Status.NbdDevice != nil {
		err := commands.NbdClientDisconnect(*deviceSwitch.Status.NbdDevice)
		if err != nil {
			return err
		}
	}

	controllerutil.RemoveFinalizer(deviceSwitch, config.Finalizer)

	if err := r.Update(ctx, deviceSwitch); err != nil {
		return err
	}

	return nil
}

func (r *DeviceSwitchNodeReconciler) createDmMultipath(name string, sizeBytes int64, inputDevPath *string) (string, error) {
	upperDevName := fmt.Sprintf("%s-upper", name)
	lowerDevName := fmt.Sprintf("%s-lower", name)

	upperDevPath := fmt.Sprintf("/dev/mapper/%s", upperDevName)
	lowerDevPath := fmt.Sprintf("/dev/mapper/%s", lowerDevName)

	upperDevTable := upperTable(sizeBytes, lowerDevPath)
	lowerDevTable := lowerTable(sizeBytes, inputDevPath)

	_, err := commands.DmsetupCreateIdempotent(lowerDevName, "--table", lowerDevTable)
	if err != nil {
		return "", err
	}

	_, err = commands.Dmsetup("mknodes", lowerDevName)
	if err != nil {
		return "", err
	}

	_, err = commands.DmsetupCreateIdempotent(upperDevName, "--table", upperDevTable)
	if err != nil {
		return "", err
	}

	_, err = commands.Dmsetup("mknodes", upperDevName)
	if err != nil {
		return "", err
	}

	return upperDevPath, nil
}

func (r *DeviceSwitchNodeReconciler) removeDmMultipath(name string) error {
	upperDevName := fmt.Sprintf("%s-upper", name)
	lowerDevName := fmt.Sprintf("%s-lower", name)

	// --force replaces table with error target, which prevents hangs when removing a disconnected volume

	_, err := commands.DmsetupRemoveIdempotent(upperDevName, "--force")
	if err != nil {
		return err
	}

	_, err = commands.DmsetupRemoveIdempotent(lowerDevName, "--force")
	if err != nil {
		return err
	}

	return nil
}

func (r *DeviceSwitchNodeReconciler) reconfigureDmMultipath(name string, sizeBytes int64, inputDevPath *string) error {
	upperDevName := fmt.Sprintf("%s-upper", name)
	lowerDevName := fmt.Sprintf("%s-lower", name)

	lowerDevPath := fmt.Sprintf("/dev/mapper/%s", lowerDevName)

	upperDevTable := upperTable(sizeBytes, lowerDevPath)
	lowerDevTable := lowerTable(sizeBytes, inputDevPath)

	_, err := commands.Dmsetup("message", upperDevName, "0", fmt.Sprintf("fail_path %s", lowerDevPath))
	if err != nil {
		return err
	}

	if err := dmsetupSuspendLoadResume(lowerDevName, lowerDevTable); err != nil {
		return err
	}

	if err := dmsetupSuspendLoadResume(upperDevName, upperDevTable); err != nil {
		return err
	}

	_, err = commands.Dmsetup("message", upperDevName, "0", fmt.Sprintf("reinstate_path %s", lowerDevPath))
	if err != nil {
		return err
	}

	return nil
}

func dmsetupSuspendLoadResume(devName string, table string) error {
	_, err := commands.Dmsetup("suspend", devName) // flush any in-flight I/O
	if err != nil {
		return err
	}

	_, err = commands.Dmsetup("load", devName, "--table", table)
	if err != nil {
		return err
	}

	_, err = commands.Dmsetup("resume", devName)
	if err != nil {
		return err
	}

	return nil
}

func upperTable(sizeBytes int64, lowerDevPath string) string {
	return fmt.Sprintf("0 %d multipath 3 queue_if_no_path queue_mode bio 0 1 1 round-robin 0 1 0 %s", sizeBytes/512, lowerDevPath)
}

func lowerTable(sizeBytes int64, inputDevPath *string) string {
	if inputDevPath != nil {
		return fmt.Sprintf("0 %d linear %s 0", sizeBytes/512, *inputDevPath)
	} else {
		return fmt.Sprintf("0 %d error", sizeBytes/512)
	}
}
