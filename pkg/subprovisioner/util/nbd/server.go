// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net"
	"time"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

//go:embed server-replicaset.template.yaml
var serverReplicaSetYamlTemplateFile string
var serverReplicaSetYamlTemplate = template.Must(template.New("").Parse(serverReplicaSetYamlTemplateFile))

//go:embed server-service.template.yaml
var serverServiceYamlTemplateFile string
var serverServiceYamlTemplate = template.Must(template.New("").Parse(serverServiceYamlTemplateFile))

type ServerId struct {
	// The node to which the server should be scheduled.
	NodeName string

	BlobName string
}

func (id *ServerId) Hostname() string {
	return fmt.Sprintf("nbd-server-%s", id.hash())
}

func (id *ServerId) hash() string {
	return util.Hash(id.NodeName, id.BlobName)
}

func (id *ServerId) ResolveHost() (net.IP, error) {
	hostname := id.Hostname()

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("could not resolve hostname '%s'", hostname)
	}

	return ips[0], nil
}

// Returns only once the server is running and has the TCP port open.
func StartServer(ctx context.Context, clientset kubernetes.Interface, id *ServerId, devicePathOnHost string) error {
	// create  Service

	service := &corev1.Service{}
	err := instantiateTemplate(serverServiceYamlTemplate, id, devicePathOnHost, service)
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().Services(config.K8sNamespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	// create  ReplicaSet

	replicaSet := &appsv1.ReplicaSet{}
	err = instantiateTemplate(serverReplicaSetYamlTemplate, id, devicePathOnHost, replicaSet)
	if err != nil {
		return err
	}

	replicaSets := clientset.AppsV1().ReplicaSets(config.K8sNamespace)

	_, err = replicaSets.Create(ctx, replicaSet, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	// wait until  ReplicaSet is ready

	// TODO: Watch instead of polling.
	for {
		replicaSet, err := replicaSets.Get(ctx, replicaSet.Name, metav1.GetOptions{})

		if err != nil {
			return err
		} else if replicaSet.Status.ReadyReplicas > 0 {
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err()
		}

		time.Sleep(1 * time.Second)
	}
}

func StopServer(ctx context.Context, clientset kubernetes.Interface, id *ServerId) error {
	name := id.Hostname()

	// delete server Service

	services := clientset.CoreV1().Services(config.K8sNamespace)

	// TODO: Watch instead of polling.
	for {
		propagation := metav1.DeletePropagationForeground
		err := services.Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})

		if k8serrors.IsNotFound(err) {
			break
		} else if err != nil {
			return err
		} else if ctx.Err() != nil {
			return ctx.Err()
		}

		time.Sleep(1 * time.Second)
	}

	// delete server ReplicaSet

	replicaSets := clientset.AppsV1().ReplicaSets(config.K8sNamespace)

	// TODO: Watch instead of polling.
	for {
		propagation := metav1.DeletePropagationForeground
		err := replicaSets.Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})

		if k8serrors.IsNotFound(err) {
			break
		} else if err != nil {
			return err
		} else if ctx.Err() != nil {
			return ctx.Err()
		}

		time.Sleep(1 * time.Second)
	}

	// success

	return nil
}

func instantiateTemplate(
	yamlTemplate *template.Template,
	id *ServerId,
	devicePathOnHost string,
	object runtime.Object,
) error {
	args := map[string]template.HTML{
		"Name":             template.HTML(id.Hostname()),
		"NodeName":         template.HTML(id.NodeName),
		"Image":            template.HTML(config.Image),
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
