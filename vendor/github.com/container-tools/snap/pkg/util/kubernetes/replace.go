package kubernetes

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var skipReplace = map[string]bool{
	"PersistentVolumeClaim": true,
}

// ReplaceResources allows to completely replace a list of resources on Kubernetes, taking care of immutable fields and resource versions
func ReplaceResources(ctx context.Context, c ctrl.Client, objects []runtime.Object) error {
	for _, object := range objects {
		err := ReplaceResource(ctx, c, object)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReplaceResourceInNamespace sets the namespace before replacing
func ReplaceResourceInNamespace(ctx context.Context, c ctrl.Client, object runtime.Object, ns string) error {
	if meta, ok := object.(metav1.Object); ok {
		meta.SetNamespace(ns)
	}
	return ReplaceResource(ctx, c, object)
}

// ReplaceResource allows to completely replace a resource on Kubernetes, taking care of immutable fields and resource versions
func ReplaceResource(ctx context.Context, c ctrl.Client, res runtime.Object) error {
	err := c.Create(ctx, res)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		var typeInfo meta.Type
		typeInfo, err = meta.TypeAccessor(res)
		if err != nil {
			return err
		}
		if skipReplace[typeInfo.GetKind()] {
			return nil
		}
		existing := res.DeepCopyObject()
		var key k8sclient.ObjectKey
		key, err = k8sclient.ObjectKeyFromObject(existing)
		if err != nil {
			return err
		}
		err = c.Get(ctx, key, existing)
		if err != nil {
			return err
		}
		mapRequiredMeta(existing, res)
		mapRequiredServiceData(existing, res)
		err = c.Update(ctx, res)
	}
	if err != nil {
		return errors.Wrap(err, "could not create or replace "+findResourceDetails(res))
	}
	return nil
}

func mapRequiredMeta(from runtime.Object, to runtime.Object) {
	if fromC, ok := from.(metav1.Object); ok {
		if toC, ok := to.(metav1.Object); ok {
			toC.SetResourceVersion(fromC.GetResourceVersion())
		}
	}
}

func mapRequiredServiceData(from runtime.Object, to runtime.Object) {
	if fromC, ok := from.(*corev1.Service); ok {
		if toC, ok := to.(*corev1.Service); ok {
			toC.Spec.ClusterIP = fromC.Spec.ClusterIP
		}
	}
}

func findResourceDetails(res runtime.Object) string {
	if res == nil {
		return "nil resource"
	}
	if meta, ok := res.(metav1.Object); ok {
		name := meta.GetName()
		if ty, ok := res.(metav1.Type); ok {
			return ty.GetKind() + " " + name
		}
		return "resource " + name
	}
	return "unnamed resource"
}
