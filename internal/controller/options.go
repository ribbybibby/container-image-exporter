package controller

import (
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Option is a functional option that configures a controller
type Option func(*options)

type options struct {
	k8sKeychain   bool
	cacheDuration time.Duration
	platform      *v1.Platform
}

// WithCacheDuration is a functional option that configures the amount of time
// the controller will cache image details before making another request to the
// registry
func WithCacheDuration(d time.Duration) Option {
	return func(o *options) {
		if d <= 0 {
			return
		}
		o.cacheDuration = d
	}
}

// WithK8sKeychain is a functional option that configures whether the controller
// will fetch credentials from pull secrets in the cluster
func WithK8sKeychain(k8sKeychain bool) Option {
	return func(o *options) {
		o.k8sKeychain = k8sKeychain
	}
}

// WithPlatform is a functional option that configures the default platform that
// the conrtroller will resolve multi-architecture images to
func WithPlatform(platform *v1.Platform) Option {
	return func(o *options) {
		o.platform = platform
	}
}
