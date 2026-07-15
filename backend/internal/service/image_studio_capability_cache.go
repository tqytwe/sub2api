package service

import (
	"strings"
	"sync"
)

type ImageStudioCapabilityCache struct {
	mu      sync.RWMutex
	denials map[string]struct{}
}

func NewImageStudioCapabilityCache() *ImageStudioCapabilityCache {
	return &ImageStudioCapabilityCache{
		denials: make(map[string]struct{}),
	}
}

func imageStudioCapabilityKey(model, size string) string {
	return strings.ToLower(strings.TrimSpace(model)) + "\x00" + strings.TrimSpace(size)
}

func (c *ImageStudioCapabilityCache) Deny(model, size string) {
	model = strings.TrimSpace(model)
	size = strings.TrimSpace(size)
	if model == "" || size == "" {
		return
	}
	c.mu.Lock()
	c.denials[imageStudioCapabilityKey(model, size)] = struct{}{}
	c.mu.Unlock()
}

func (c *ImageStudioCapabilityCache) IsDenied(model, size string) bool {
	model = strings.TrimSpace(model)
	size = strings.TrimSpace(size)
	if model == "" || size == "" {
		return false
	}
	c.mu.RLock()
	_, ok := c.denials[imageStudioCapabilityKey(model, size)]
	c.mu.RUnlock()
	return ok
}
