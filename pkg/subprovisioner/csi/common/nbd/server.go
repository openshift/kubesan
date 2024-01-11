// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"bytes"
	"context"
	"crypto/sha1"
	_ "embed"
	"fmt"
	"html/template"
	"time"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/k8s"
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
var serverReplicaSetYamlTemplate = template.Must(template.New("nbd-server").Parse(serverReplicaSetYamlTemplateFile))

//go:embed server-service.template.yaml
var serverServiceYamlTemplateFile string
var serverServiceYamlTemplate = template.Must(template.New("nbd-server").Parse(serverServiceYamlTemplateFile))

type ServerConfig struct {
	// The UID of the PVC of the volume being exported.
	PvcUid types.UID

	// The node to which the server should be scheduled.
	NodeName string

	// The host path to the device to be exported.
	DevicePathOnHost string

	// The container image to be run as the server.
	Image string
}

func (s *ServerConfig) Hostname() string {
	return s.getName()
}

// Returns only once the server is running and has the TCP port open.
func (s *ServerConfig) StartServer(ctx context.Context, c *k8s.Clientset) error {
	// create  Service

	service := &corev1.Service{}
	err := s.instantiateTemplate(serverServiceYamlTemplate, service)
	if err != nil {
		return err
	}

	_, err = c.Clientset.CoreV1().Services(config.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	// create  ReplicaSet

	replicaSet := &appsv1.ReplicaSet{}
	err = s.instantiateTemplate(serverReplicaSetYamlTemplate, replicaSet)
	if err != nil {
		return err
	}

	replicaSets := c.Clientset.AppsV1().ReplicaSets(config.Namespace)

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

func (s *ServerConfig) StopServer(ctx context.Context, c *k8s.Clientset) error {
	name := s.getName()

	// delete server Service

	services := c.Clientset.CoreV1().Services(config.Namespace)

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

	replicaSets := c.Clientset.AppsV1().ReplicaSets(config.Namespace)

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

func (s *ServerConfig) getName() string {
	// Node object names are DNS Subdomain Names, and so can be up to 253 characters in length, which means we can't
	// embed s.NodeName directly in the object name we return here. But we also don't want to use the Node object's
	// UID, just in case the Node object is recreated with the same name for some reason but still refers to the
	// same actual node in the cluster. We get around this by hashing s.PvcUid and s.NodeName and basing the name on
	// that.

	hash := sha1.New()
	hash.Write([]byte(s.PvcUid))
	hash.Write([]byte(s.NodeName))

	return fmt.Sprintf("nbd-server-%x", hash.Sum(nil))
}

func (s *ServerConfig) instantiateTemplate(yamlTemplate *template.Template, object runtime.Object) error {
	args := map[string]string{
		"Name":             s.getName(),
		"NodeName":         s.NodeName,
		"Image":            s.Image,
		"DevicePathOnHost": s.DevicePathOnHost,
	}

	var serverYaml bytes.Buffer
	err := yamlTemplate.Execute(&serverYaml, args)
	if err != nil {
		return err
	}

	_, _, err = scheme.Codecs.UniversalDeserializer().Decode(serverYaml.Bytes(), nil, object)
	return err
}
