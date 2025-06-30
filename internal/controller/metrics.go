package controller

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "container_image"
)

var (
	metricContainerInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "container_info"),
		"img about containers running in the cluster, including the image digest resolved by the exporter.",
		[]string{"group", "version", "kind", "namespace", "name", "jsonpath", "image", "digest"}, nil,
	)
	metricAnnotation = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "annotation"),
		"Annotations from the image manifest.",
		[]string{"digest", "key", "value"}, nil,
	)
	metricLabel = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "label"),
		"Labels from the image config.",
		[]string{"digest", "key", "value"}, nil,
	)
	metricSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "size_bytes"),
		"The size of the image in the registry.",
		[]string{"digest"}, nil,
	)
	metricCreated = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "created"),
		"The created date from the image config. Expressed as a Unix Epoch Time.",
		[]string{"digest"}, nil,
	)
)

// Exporter exports metrics about container images in Kubernetes
type Exporter struct {
	client client.Client
	cache  ContainerImageCache
}

// NewExporter constructs a new exporter
func NewExporter(c client.Client, cache ContainerImageCache) *Exporter {
	return &Exporter{
		client: c,
		cache:  cache,
	}
}

// Describe all the metrics provided by the Exporter
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- metricContainerInfo
	ch <- metricAnnotation
	ch <- metricLabel
	ch <- metricSize
	ch <- metricCreated
}

// Collect metrics
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	digests := map[string]struct{}{}
	for _, resource := range resources {
		ul := &unstructured.UnstructuredList{}
		ul.SetGroupVersionKind(resource.GroupVersionKind)

		if err := e.client.List(context.Background(), ul); err != nil {
			return
		}

		for _, item := range ul.Items {
			for _, container := range containerSpecs(&item) {
				// Fetch image from the cache
				var digestStr string
				img, err := e.fetchImage(ctx, container.Image)
				if err == nil {
					digestStr = img.Digest
				}

				ch <- prometheus.MustNewConstMetric(
					metricContainerInfo,
					prometheus.GaugeValue,
					1.0,
					item.GroupVersionKind().Group,
					item.GroupVersionKind().Version,
					item.GroupVersionKind().Kind,
					item.GetNamespace(),
					item.GetName(),
					container.JSONPath,
					container.Image,
					digestStr,
				)

				// We can only collect image-specific metrics if
				// we could fetch the image metadata
				if img == nil {
					continue
				}

				// Only process digest-specific metrics once
				if _, ok := digests[img.Digest]; ok {
					continue
				}
				digests[img.Digest] = struct{}{}

				ch <- prometheus.MustNewConstMetric(
					metricSize, prometheus.GaugeValue, float64(img.Size), img.Digest,
				)
				ch <- prometheus.MustNewConstMetric(
					metricCreated, prometheus.GaugeValue, float64(img.Created.Unix()), img.Digest,
				)

				for k, v := range img.Annotations {
					ch <- prometheus.MustNewConstMetric(
						metricAnnotation,
						prometheus.GaugeValue,
						1.0,
						img.Digest,
						k,
						v,
					)
				}
				for k, v := range img.Labels {
					ch <- prometheus.MustNewConstMetric(
						metricLabel,
						prometheus.GaugeValue,
						1.0,
						img.Digest,
						k,
						v,
					)
				}

			}
		}
	}
}

func (e *Exporter) fetchImage(ctx context.Context, imgRef string) (*ContainerImage, error) {
	ref, err := name.ParseReference(imgRef)
	if err != nil {
		return nil, fmt.Errorf("parsing image: %w", err)
	}

	img, err := e.cache.Get(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("fetching image from the cache: %w", err)
	}

	return img.ContainerImage, nil
}
