// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"html/template"
	"strconv"
	"time"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

type Job struct {
	Name        string
	NodeName    string
	Command     []string
	HostNetwork bool
}

// Idempotent, until you call Delete() on the job.
func Run(ctx context.Context, clientset *k8s.Clientset, job *Job) error {
	// create job

	jobObject, err := job.instantiateTemplate()
	if err != nil {
		return err
	}

	jobs := clientset.BatchV1().Jobs(config.K8sNamespace)

	_, err = jobs.Create(ctx, jobObject, v1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	// wait until job succeeds

	// TODO: Watch instead of polling.
	for {
		jobObject, err := jobs.Get(ctx, job.Name, v1.GetOptions{})

		if jobObject.Status.Succeeded > 0 {
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

// Idempotent.
func Delete(ctx context.Context, clientset *k8s.Clientset, jobName string) error {
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
		"Name":        template.HTML(job.Name),
		"NodeName":    template.HTML(job.NodeName),
		"Image":       template.HTML(config.Image),
		"CommandJson": template.HTML(commandJson),
		"HostNetwork": template.HTML(strconv.FormatBool(job.HostNetwork)),
	}

	var jobYaml bytes.Buffer
	err = jobYamlTemplate.Execute(&jobYaml, args)
	if err != nil {
		return nil, err
	}

	object, _, err := scheme.Codecs.UniversalDeserializer().Decode(jobYaml.Bytes(), nil, nil)
	if err != nil {
		return nil, err
	}

	return object.(*batchv1.Job), nil
}
