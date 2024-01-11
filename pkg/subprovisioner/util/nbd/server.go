// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"bytes"
	"context"
	"crypto/sha1"
	_ "embed"
	"fmt"
	"html/template"
	"net"
	"time"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	// The UID of the PVC of the volume being exported.
	PvcUid types.UID
}

func (id *ServerId) Hostname() string {
	// Node object names are DNS Subdomain Names, and so can be up to 253 characters in length, which means we can't
	// embed id.NodeName directly in the object name we return here. But we also don't want to use the Node object'id
	// UID, just in case the Node object is recreated with the same name for some reason but still refers to the
	// same actual node in the cluster. We get around this by hashing id.PvcUid and id.NodeName and basing the name on
	// that.

	hash := sha1.New()
	hash.Write([]byte(id.NodeName))
	hash.Write([]byte(id.PvcUid))

	return fmt.Sprintf("nbd-server-%x", hash.Sum(nil))
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
func StartServer(ctx context.Context, clientset *k8s.Clientset, id *ServerId, devicePathOnHost string) error {
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

func StopServer(ctx context.Context, clientset *k8s.Clientset, id *ServerId) error {
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

	var serverYaml bytes.Buffer
	err := yamlTemplate.Execute(&serverYaml, args)
	if err != nil {
		return err
	}

	_, _, err = scheme.Codecs.UniversalDeserializer().Decode(serverYaml.Bytes(), nil, object)
	return err
}
