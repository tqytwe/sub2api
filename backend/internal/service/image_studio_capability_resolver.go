package service

import (
	"strings"
)

func allImageStudioCatalogSizes() []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(imageStudioSizeMatrix)*3)
	for _, aspect := range imageStudioAspectCatalog {
		tiers, ok := imageStudioSizeMatrix[aspect.ID]
		if !ok {
			continue
		}
		for _, tier := range imageStudioTierCatalog {
			if tier.ID == ImageStudioTier3K {
				continue
			}
			size, ok := tiers[tier.ID]
			if !ok || strings.TrimSpace(size) == "" {
				continue
			}
			if _, exists := seen[size]; exists {
				continue
			}
			seen[size] = struct{}{}
			out = append(out, size)
		}
	}
	return out
}

func agnesImageStudioSizes() []string {
	out := make([]string, 0, len(imageStudioAspectCatalog)*len(imageStudioTierCatalog))
	for _, aspect := range imageStudioAspectCatalog {
		tiers, ok := imageStudioSizeMatrix[aspect.ID]
		if !ok {
			continue
		}
		for _, tier := range []string{ImageBillingSize1K, ImageBillingSize2K, ImageStudioTier3K, ImageBillingSize4K} {
			if size := strings.TrimSpace(tiers[tier]); size != "" {
				out = append(out, size)
			}
		}
	}
	return out
}

func inferImageStudioQualities(model string) []string {
	if capability, ok := ResolveImageStudioModelCapability(model); ok {
		return append([]string(nil), capability.SupportedQualities...)
	}
	return nil
}

func isImageStudioSizeRelatedError(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	if msg == "" {
		return false
	}
	for _, kw := range []string{
		"size",
		"dimension",
		"resolution",
		"invalid_image",
		"image_size",
		"aspect",
		"too large",
		"not supported",
	} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

func (s *ImageStudioService) ResolveModelCapabilities(_ *APIKey, model string) ImageStudioModelCapabilities {
	capability, ok := ResolveImageStudioModelCapability(model)
	if !ok {
		return ImageStudioModelCapabilities{}
	}
	capability.SupportedSizes = filterImageStudioSizesForModel(s, model, capability.SupportedSizes)
	return capability
}

func filterImageStudioSizesForModel(s *ImageStudioService, model string, sizes []string) []string {
	if len(sizes) == 0 {
		return nil
	}
	if s == nil || s.capabilityCache == nil {
		return append([]string(nil), sizes...)
	}
	out := make([]string, 0, len(sizes))
	for _, size := range sizes {
		if s.capabilityCache.IsDenied(model, size) {
			continue
		}
		out = append(out, size)
	}
	return out
}

func (s *ImageStudioService) ValidateSizeForModel(apiKey *APIKey, model, size string) error {
	size = strings.TrimSpace(size)
	if size == "" {
		return ErrImageStudioSizeNotSupported
	}
	if !isKnownImageStudioSize(size) {
		return ErrImageStudioSizeNotSupported
	}
	for _, supported := range s.ResolveModelCapabilities(apiKey, model).SupportedSizes {
		if supported == size {
			return nil
		}
	}
	return ErrImageStudioSizeNotSupported
}

func (s *ImageStudioService) ValidateQualityForModel(model, quality string) error {
	quality = strings.TrimSpace(strings.ToLower(quality))
	if quality == "" {
		return nil
	}
	supported := inferImageStudioQualities(model)
	if len(supported) == 0 {
		return nil
	}
	for _, item := range supported {
		if item == quality {
			return nil
		}
	}
	return ErrImageStudioQualityNotSupported
}

func (s *ImageStudioService) RecordGenerateFailure(model, size, errMsg string) {
	if s == nil || s.capabilityCache == nil {
		return
	}
	if !isImageStudioSizeRelatedError(errMsg) {
		return
	}
	s.capabilityCache.Deny(model, size)
}
