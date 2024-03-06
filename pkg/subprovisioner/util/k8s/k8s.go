// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

type ObjectWithMeta interface {
	runtime.Object
	metav1.ObjectMetaAccessor
}

func AtomicUpdate[T ObjectWithMeta](
	ctx context.Context, rest rest.Interface,
	resource string, object T,
	f func(T) error,
) error {
	name := object.GetObjectMeta().GetName()
	namespace := object.GetObjectMeta().GetNamespace()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// get object

		req := rest.Get().Resource(resource).Name(name).NamespaceIfScoped(namespace, namespace != "").
			VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec)

		err := req.Do(ctx).Into(object)
		if err != nil {
			return err
		}

		// mutate object

		err = f(object)
		if err != nil {
			return err
		}

		// update object

		req = rest.Put().Resource(resource).Name(name).NamespaceIfScoped(namespace, namespace != "").
			Body(object).VersionedParams(&metav1.UpdateOptions{}, scheme.ParameterCodec)

		err = req.Do(ctx).Into(object)
		if err != nil {
			return err
		}

		return nil
	})
}
