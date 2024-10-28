// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"

	"gitlab.com/kubesan/kubesan/internal/common/config"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubesanslices "gitlab.com/kubesan/kubesan/internal/common/slices"
	"gitlab.com/kubesan/kubesan/internal/manager/common/util"
)

//go:embed server-pod.template.yaml
var serverPodYamlTemplateFile string
var serverPodYamlTemplate = template.Must(template.New("").Parse(serverPodYamlTemplateFile))

// The NbdExport CR should be named Node-Export; but to make it easier,
// this code takes both pieces as separate items.
type ServerId struct {
	// The node to which the server should be scheduled.
	Node string

	// The export name
	Export string
}

func (id *ServerId) Podname() string {
	// The NbdExport CRD ensured that Node and Export match [-_.a-z0-9]+
	return fmt.Sprintf("nbd-server-%s-%s", id.Node, id.Export)
}

// Returns success only once the server is running and has the TCP port open.
// An error return of WatchPending indicates that the caller should try this
// function again the next time the resource is reconciled. Make sure to add
// mgr.Owns(corev1.Pod) when creating the controller-runtime manager, so that
// the necessary Watch will spot the desired Pod resource changes.
func StartServer(ctx context.Context, owner metav1.Object, scheme *runtime.Scheme, c client.Client, id *ServerId, devicePathOnHost string) (string, error) {
	// create Pod

	image, err := config.GetImage(ctx, c)
	if err != nil {
		return "", err
	}

	pod := &corev1.Pod{}
	err = instantiateTemplate(serverPodYamlTemplate, id, devicePathOnHost, pod, image)
	if err != nil {
		return "", err
	}

	if err = controllerutil.SetControllerReference(owner, pod, scheme); err != nil {
		return "", err
	}

	err = c.Create(ctx, pod)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return "", err
	}

	// check if Pod is ready

	err = c.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: config.Namespace}, pod)
	if err != nil {
		return "", err
	}

	ready := kubesanslices.Any(pod.Status.Conditions, func(cond corev1.PodCondition) bool {
		return cond.Type == "Ready" && cond.Status == "True"
	})
	if ready {
		return fmt.Sprintf("nbd://%s/%s",
			pod.Status.PodIP, id.Export), nil
	}
	return "", &util.WatchPending{}
}

func CheckServerHealth(ctx context.Context, c client.Client, id *ServerId) error {
	pod := &corev1.Pod{}
	err := c.Get(ctx, types.NamespacedName{Name: id.Podname(), Namespace: config.Namespace}, pod)
	if err != nil {
		return err
	}

	if pod.Status.Phase != "Running" {
		return k8serrors.NewServiceUnavailable("NBD server unexpectedly gone")
	}

	return nil
}

// An error return of WatchPending indicates that the caller should try this
// function again the next time the resource is reconciled.
func StopServer(ctx context.Context, c client.Client, id *ServerId) error {
	// delete server
	propagation := client.PropagationPolicy(metav1.DeletePropagationForeground)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      id.Podname(),
			Namespace: config.Namespace,
		},
	}
	err := c.Delete(ctx, pod, propagation)

	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err == nil {
		return &util.WatchPending{}
	}
	return err
}

func instantiateTemplate(
	yamlTemplate *template.Template,
	id *ServerId,
	devicePathOnHost string,
	object runtime.Object,
	image string,
) error {
	args := map[string]template.HTML{
		"Podname":          template.HTML(id.Podname()),
		"Namespace":        template.HTML(config.Namespace),
		"Node":             template.HTML(id.Node),
		"Image":            template.HTML(image),
		"Export":           template.HTML(id.Export),
		"DevicePathOnHost": template.HTML(devicePathOnHost),
	}

	var yaml bytes.Buffer
	err := yamlTemplate.Execute(&yaml, args)
	if err != nil {
		return err
	}

	_, _, err = scheme.Codecs.UniversalDeserializer().Decode(yaml.Bytes(), nil, object)
	if err != nil {
		return fmt.Errorf("failed to decode YAML: %s:\n%s", err, yaml.String())
	}

	return nil
}
