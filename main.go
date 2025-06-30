package main

import (
	"fmt"
	"os"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/ribbybibby/container-image-exporter/internal/controller"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
}

var (
	metricsAddr   string
	probeAddr     string
	cacheDuration time.Duration
	platform      string
)

var rootCmd = &cobra.Command{
	Use:   "container-image-exporter",
	Short: "Export metrics about container images in a Kubernetes cluster.",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := zap.Options{}

		ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			HealthProbeBindAddress: probeAddr,
		})
		if err != nil {
			return fmt.Errorf("creating a new manager: %w", err)
		}

		var p *v1.Platform
		if platform != "" {
			p, err = v1.ParsePlatform(platform)
			if err != nil {
				return fmt.Errorf("parsing platform: %w", err)
			}
		}

		if err = controller.SetupControllers(
			mgr,
			controller.WithCacheDuration(cacheDuration),
			controller.WithPlatform(p),
		); err != nil {
			return fmt.Errorf("setting up controllers: %w", err)
		}

		if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
			return fmt.Errorf("adding healthz check: %w", err)
		}
		if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
			return fmt.Errorf("adding readyz check: %w", err)
		}

		return mgr.Start(ctrl.SetupSignalHandler())
	},
}

func init() {
	rootCmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	rootCmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	rootCmd.Flags().StringVar(&platform, "platform", "linux/amd64", "The default platform to resolve multi-arch images to.")
	rootCmd.Flags().DurationVar(&cacheDuration, "cache-duration", 1*time.Hour, "How long to cache image details for before querying the registry again.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
