package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
)

// ErrContainerImageNotFound is returned when an item isn't in the cache
var ErrContainerImageNotFound = fmt.Errorf("not found")

// CachedContainerImage is a container image that we've cached
type CachedContainerImage struct {
	*ContainerImage
	Time time.Time
}

// ContainerImageCache caches details about container images
type ContainerImageCache interface {
	Get(ctx context.Context, ref name.Reference) (*CachedContainerImage, error)
	Put(ctx context.Context, ref name.Reference, img *ContainerImage) error
}

type cacheImpl struct {
	digestMap map[string]string
	imageMap  map[string]*CachedContainerImage
	lock      sync.Mutex
}

// NewContainerImageCache returns a new cache
func NewContainerImageCache() ContainerImageCache {
	return &cacheImpl{
		digestMap: map[string]string{},
		imageMap:  map[string]*CachedContainerImage{},
		lock:      sync.Mutex{},
	}
}

// Get an image from the cache
func (c *cacheImpl) Get(ctx context.Context, ref name.Reference) (*CachedContainerImage, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var digestStr string
	if digest, ok := ref.(name.Digest); ok {
		digestStr = digest.DigestStr()
	} else {
		digest, ok := c.digestMap[ref.String()]
		if ok {
			digestStr = digest
		}
	}
	if digestStr == "" {
		return nil, ErrContainerImageNotFound
	}

	img, ok := c.imageMap[digestStr]
	if !ok {
		return nil, ErrContainerImageNotFound
	}

	return img, nil
}

// Put an image into the cache
func (c *cacheImpl) Put(ctx context.Context, ref name.Reference, img *ContainerImage) error {
	if img == nil {
		return nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.digestMap[ref.String()] = img.Digest
	c.imageMap[img.Digest] = &CachedContainerImage{
		ContainerImage: img,
		Time:           time.Now(),
	}

	return nil
}
