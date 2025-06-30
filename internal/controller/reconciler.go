package controller

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8schain "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// ContainerImage describes a container image
type ContainerImage struct {
	// Digest is the digest of the image
	Digest string

	// Annotations are the annotations on the image manifest
	Annotations map[string]string

	// Labels are the labels in the image config
	Labels map[string]string

	// Size is the size of the image in the registry
	Size int64

	// Created is created time from the image config
	Created time.Time
}

// ContainerImageReconciler reconciles container images described in a
// Kubernetes object
type ContainerImageReconciler struct {
	client.Client
	KubeClient       kubernetes.Interface
	GroupVersionKind schema.GroupVersionKind
	Cache            ContainerImageCache
	CacheDuration    time.Duration
	Platform         *v1.Platform
}

// Reconcile reconciles objects that define containers
func (r *ContainerImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.Log.WithValues(
		"group", r.GroupVersionKind.Group,
		"version", r.GroupVersionKind.Version,
		"kind", r.GroupVersionKind.Kind,
		"namespace", req.Namespace,
		"name", req.Name,
	)
	logger.Info("Reconciling")

	// Get the object
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(r.GroupVersionKind)
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Construct a keychain which uses the pull secrets attached to the object
	// and/or the object's service account, as well any cloud provider
	// credentials or docker config.
	opts := k8schain.Options{
		Namespace:          obj.GetNamespace(),
		ServiceAccountName: serviceAccountName(obj),
		ImagePullSecrets:   imagePullSecrets(obj),
	}
	kc, err := k8schain.New(ctx, r.KubeClient, opts)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("constructing k8s keychain: %w", err)
	}

	// Iterate over every container spec in the object, fetching the image
	// metadata. This populates the cache that we export metrics from.
	for _, container := range containerSpecs(obj) {
		logger.Info("Fetching image metadata", "image", container.Image)
		img, err := r.getImage(ctx, container.Image, remote.WithAuthFromKeychain(kc))
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("fetching image details: %w", err)
		}
		logger.Info("Fetched image metadata", "image", container.Image, "digest", img.Digest)
	}

	// Tags are mutable so we should periodically check to see if the digest
	// of any of the container images has changed by requeueing the object.
	d := addJitter(r.CacheDuration)
	logger.Info("Reconciled", "requeue_after", d)
	return ctrl.Result{
		RequeueAfter: d,
	}, nil
}

// addJitter adds random jitter to a duration, extending it by up to 1/6 of the
// original duration. This helps spread out reconciliation times to avoid
// thundering herd problems.
func addJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	// Add jitter between 0 and d/6
	maxJitter := d / 6
	jitter := time.Duration(rand.Int63n(int64(maxJitter)))
	return d + jitter
}

func (r *ContainerImageReconciler) getImage(ctx context.Context, imgRef string, opts ...remote.Option) (*ContainerImage, error) {
	ref, err := name.ParseReference(imgRef)
	if err != nil {
		return nil, fmt.Errorf("parsing image reference: %w", err)
	}

	// If the cache is configured, attempt to get the details from that
	// first
	if r.Cache != nil {
		cimg, err := r.Cache.Get(ctx, ref)
		if err == nil && time.Now().Before(cimg.Time.Add(r.CacheDuration)) {
			return cimg.ContainerImage, nil
		}
		if !errors.Is(err, ErrContainerImageNotFound) {
			return nil, fmt.Errorf("fetching image details from cache: %w", err)
		}
	}

	desc, err := remote.Get(ref, append(opts, remote.WithContext(ctx))...)
	if err != nil {
		return nil, fmt.Errorf("getting descriptor: %s: %w", ref, err)
	}

	img, err := getImage(desc, r.Platform)
	if err != nil {
		return nil, fmt.Errorf("getting image: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("getting manifest: %w", err)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("getting config: %w", err)
	}

	sz := manifest.Config.Size
	for _, layer := range manifest.Layers {
		sz = sz + layer.Size
	}

	cimg := &ContainerImage{
		Digest:      desc.Digest.String(),
		Annotations: desc.Annotations,
		Labels:      configFile.Config.Labels,
		Size:        sz,
		Created:     configFile.Created.Time,
	}

	// If a cache is configured then cache the details
	if r.Cache != nil {
		if err := r.Cache.Put(ctx, ref, cimg); err != nil {
			return nil, fmt.Errorf("putting details for %s into the cache: %w", desc.Digest, err)
		}
	}

	return cimg, nil
}

func getImage(desc *remote.Descriptor, platform *v1.Platform) (v1.Image, error) {
	switch desc.MediaType {
	case types.OCIImageIndex, types.DockerManifestList:
		idx, err := desc.ImageIndex()
		if err != nil {
			return nil, fmt.Errorf("fetching index: %w", err)
		}
		indexManifest, err := idx.IndexManifest()
		if err != nil {
			return nil, fmt.Errorf("fetching manifest: %w", err)
		}
		if len(indexManifest.Manifests) == 0 {
			return nil, fmt.Errorf("no manifests in index: %w", err)
		}

		// If a platform is configured then look for it in the manifests
		if platform != nil {
			for _, manifest := range indexManifest.Manifests {
				if manifest.Platform.Equals(*platform) {
					return idx.Image(manifest.Digest)
				}
			}
		}

		// If not, or if the platform doesn't exist in the manifests,
		// just return the first one in the list.
		return idx.Image(indexManifest.Manifests[0].Digest)
	}

	return desc.Image()
}

// ContainerSpec is information about a container
type ContainerSpec struct {
	// JSONPath is the path to the container in the object
	JSONPath string

	// Name is the name of the container
	Name string

	// Image is the image reference
	Image string
}

func containerSpecs(obj *unstructured.Unstructured) []ContainerSpec {
	// Paths where we will find container specs in various objects
	containerPaths := [][]string{
		{
			"spec", "initContainers",
		},
		{
			"spec", "containers",
		},
		{
			"spec", "ephemeralContainers",
		},
		{
			"spec", "template", "spec", "initContainers",
		},
		{
			"spec", "template", "spec", "containers",
		},
		{
			"spec", "jobTemplate", "spec", "template", "spec", "initContainers",
		},
		{
			"spec", "jobTemplate", "spec", "template", "spec", "containers",
		},
	}
	var containerSpecs []ContainerSpec
	for _, containerPath := range containerPaths {
		containers, _, _ := unstructured.NestedSlice(obj.Object, containerPath...)
		for i, container := range containers {
			data, ok := container.(map[string]interface{})
			if !ok {
				continue
			}
			name, ok := data["name"].(string)
			if !ok {
				continue
			}
			image, ok := data["image"].(string)
			if !ok {
				continue
			}
			containerSpecs = append(containerSpecs, ContainerSpec{
				JSONPath: fmt.Sprintf("{.%s[%d]}", strings.Join(containerPath, "."), i),
				Name:     name,
				Image:    image,
			})
		}
	}

	return containerSpecs
}

func imagePullSecrets(obj *unstructured.Unstructured) []string {
	imagePullSecretsPaths := [][]string{
		{
			"spec", "imagePullSecrets",
		},
		{
			"spec", "template", "spec", "imagePullSecrets",
		},
		{
			"spec", "jobTemplate", "spec", "template", "spec", "imagePullSecrets",
		},
	}

	var secrets []string
	for _, imagePullSecretsPath := range imagePullSecretsPaths {
		pullSecrets, _, _ := unstructured.NestedSlice(obj.Object, imagePullSecretsPath...)
		for _, pullSecret := range pullSecrets {
			data, ok := pullSecret.(map[string]interface{})
			if !ok {
				continue
			}
			name, ok := data["name"].(string)
			if !ok {
				continue
			}

			secrets = append(secrets, name)
		}
	}

	return secrets
}

func serviceAccountName(obj *unstructured.Unstructured) string {
	serviceAccountNamePaths := [][]string{
		{
			"spec", "serviceAccountName",
		},
		{
			"spec", "template", "spec", "serviceAccountName",
		},
		{
			"spec", "jobTemplate", "spec", "template", "spec", "serviceAccountName",
		},
	}

	for _, serviceAccountNamePath := range serviceAccountNamePaths {
		serviceAccountName, _, _ := unstructured.NestedString(obj.Object, serviceAccountNamePath...)
		if serviceAccountName != "" {
			return serviceAccountName
		}
	}

	return ""
}
