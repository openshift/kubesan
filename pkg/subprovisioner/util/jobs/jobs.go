// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"strconv"
	"time"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	batchv1 "k8s.io/api/batch/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

type Job struct {
	Name               string
	NodeName           string
	Command            []string
	HostNetwork        bool
	HostPID            bool
	ServiceAccountName string
}

func CreateAndRunAndDelete(ctx context.Context, clientset kubernetes.Interface, job *Job) error {
	err := CreateAndRun(ctx, clientset, job)
	if err != nil {
		return err
	}

	err = Delete(ctx, clientset, job.Name)
	if err != nil {
		return err
	}

	return nil
}

// Idempotent, until you call Delete() on the job.
func CreateAndRun(ctx context.Context, clientset kubernetes.Interface, job *Job) error {
	if job.NodeName == config.LocalNodeName && !job.HostNetwork {
		return runLocal(job)
	} else {
		return runRemote(ctx, clientset, job)
	}
}

func runLocal(job *Job) error {
	err := util.RunCommand(job.Command...)
	if err != nil {
		return status.Errorf(codes.Internal, "job \"%s\" failed: %s", job.Name, err)
	}

	return nil
}

func runRemote(ctx context.Context, clientset kubernetes.Interface, job *Job) error {
	// create job

	jobObject, err := job.instantiateTemplate()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to generate YAML for job \"%s\": %s", job.Name, err)
	}

	jobs := clientset.BatchV1().Jobs(config.K8sNamespace)

	_, err = jobs.Create(ctx, jobObject, v1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return status.Errorf(codes.Internal, "failed to create job \"%s\": %s", job.Name, err)
	}

	// wait until job succeeds

	// TODO: Watch instead of polling.
	for {
		jobObject, err := jobs.Get(ctx, job.Name, v1.GetOptions{})

		if jobObject.Status.Succeeded > 0 {
			break
		} else if err != nil {
			return status.Errorf(codes.Internal, "failed to get job \"%s\": %s", job.Name, err)
		} else if ctx.Err() != nil {
			return ctx.Err()
		}

		time.Sleep(1 * time.Second)
	}

	// success

	return nil
}

// Idempotent.
func Delete(ctx context.Context, clientset kubernetes.Interface, jobName string) error {
	jobs := clientset.BatchV1().Jobs(config.K8sNamespace)

	propagation := metav1.DeletePropagationForeground
	err := jobs.Delete(ctx, jobName, v1.DeleteOptions{PropagationPolicy: &propagation})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}

//go:embed job.template.yaml
var jobYamlTemplateFile string
var jobYamlTemplate = template.Must(template.New("").Parse(jobYamlTemplateFile))

func (job *Job) instantiateTemplate() (*batchv1.Job, error) {
	commandJson, err := json.Marshal(job.Command)
	if err != nil {
		return nil, err
	}

	args := map[string]template.HTML{
		"Name":               template.HTML(job.Name),
		"NodeName":           template.HTML(job.NodeName),
		"Image":              template.HTML(config.Image),
		"CommandJson":        template.HTML(commandJson),
		"HostNetwork":        template.HTML(strconv.FormatBool(job.HostNetwork)),
		"HostPID":            template.HTML(strconv.FormatBool(job.HostPID)),
		"ServiceAccountName": template.HTML(job.ServiceAccountName),
	}

	var jobYaml bytes.Buffer
	err = jobYamlTemplate.Execute(&jobYaml, args)
	if err != nil {
		return nil, err
	}

	object, _, err := scheme.Codecs.UniversalDeserializer().Decode(jobYaml.Bytes(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %s:\n%s", err, jobYaml.String())
	}

	return object.(*batchv1.Job), nil
}
