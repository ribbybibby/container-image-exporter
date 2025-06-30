package controller

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var resources = []struct {
	Object           client.Object
	GroupVersionKind schema.GroupVersionKind
}{
	{
		Object: &corev1.Pod{},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		},
	},
	{
		Object: &appsv1.Deployment{},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		},
	},
	{
		Object: &appsv1.StatefulSet{},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "StatefulSet",
		},
	},
	{
		Object: &appsv1.DaemonSet{},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "DaemonSet",
		},
	},
	{
		Object: &batchv1.Job{},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "batch",
			Version: "v1",
			Kind:    "Job",
		},
	},
	{
		Object: &batchv1.CronJob{},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "batch",
			Version: "v1",
			Kind:    "CronJob",
		},
	},
}

// SetupControllers constructs and registers controllers
func SetupControllers(mgr ctrl.Manager, opts ...Option) error {
	o := &options{
		cacheDuration: 1 * time.Hour,
	}
	for _, opt := range opts {
		opt(o)
	}

	// We use this Kubernetes client to fetch pull secrets
	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	// Avoid requesting information about the same images multiple times by
	// caching the responses.
	cache := NewContainerImageCache()
	for _, resource := range resources {
		reconciler := &ContainerImageReconciler{
			Client:           mgr.GetClient(),
			KubeClient:       kubeClient,
			GroupVersionKind: resource.GroupVersionKind,
			Cache:            cache,
			CacheDuration:    o.cacheDuration,
			Platform:         o.platform,
		}
		if err := ctrl.NewControllerManagedBy(mgr).For(resource.Object).Complete(reconciler); err != nil {
			return fmt.Errorf("unable to create controller for %s: %w", resource.GroupVersionKind, err)
		}
	}

	// Register an exporter with the controller-runtime Prometheus registry
	metrics.Registry.Register(NewExporter(mgr.GetClient(), cache))

	return nil
}
