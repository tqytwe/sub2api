package service

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrImageStudioSizeNotSupported = infraerrors.BadRequest(
	"IMAGE_STUDIO_SIZE_NOT_SUPPORTED",
	"image size is not supported for the selected model",
)

var ErrImageStudioQualityNotSupported = infraerrors.BadRequest(
	"IMAGE_STUDIO_QUALITY_NOT_SUPPORTED",
	"image quality is not supported for the selected model",
)

const ImageStudioTier3K = "3K"

type ImageStudioLocalizedLabel struct {
	Zh string `json:"zh"`
	En string `json:"en"`
}

type ImageStudioAspectOption struct {
	ID    string                    `json:"id"`
	Label ImageStudioLocalizedLabel `json:"label"`
}

type ImageStudioTierOption struct {
	ID    string                    `json:"id"`
	Label ImageStudioLocalizedLabel `json:"label"`
}

type ImageStudioSizeOption struct {
	Aspect      string `json:"aspect"`
	Tier        string `json:"tier"`
	Size        string `json:"size"`
	BillingTier string `json:"billing_tier"`
}

type ImageStudioCapabilities struct {
	Aspects     []ImageStudioAspectOption `json:"aspects"`
	Tiers       []ImageStudioTierOption   `json:"tiers"`
	SizeOptions []ImageStudioSizeOption   `json:"size_options"`
}

var imageStudioAspectCatalog = []ImageStudioAspectOption{
	{ID: "1:1", Label: ImageStudioLocalizedLabel{Zh: "正方 1:1", En: "Square 1:1"}},
	{ID: "2:3", Label: ImageStudioLocalizedLabel{Zh: "竖版 2:3", En: "Portrait 2:3"}},
	{ID: "3:2", Label: ImageStudioLocalizedLabel{Zh: "横版 3:2", En: "Landscape 3:2"}},
	{ID: "9:16", Label: ImageStudioLocalizedLabel{Zh: "竖屏 9:16", En: "Story 9:16"}},
	{ID: "16:9", Label: ImageStudioLocalizedLabel{Zh: "宽屏 16:9", En: "Wide 16:9"}},
}

var imageStudioTierCatalog = []ImageStudioTierOption{
	{ID: ImageBillingSize1K, Label: ImageStudioLocalizedLabel{Zh: "标准 1K", En: "Standard 1K"}},
	{ID: ImageBillingSize2K, Label: ImageStudioLocalizedLabel{Zh: "高清 2K", En: "HD 2K"}},
	{ID: ImageStudioTier3K, Label: ImageStudioLocalizedLabel{Zh: "精细 3K", En: "Fine 3K"}},
	{ID: ImageBillingSize4K, Label: ImageStudioLocalizedLabel{Zh: "超清 4K", En: "Ultra 4K"}},
}

var imageStudioSizeMatrix = map[string]map[string]string{
	"1:1": {
		ImageBillingSize1K: "1024x1024",
		ImageBillingSize2K: "2048x2048",
		ImageStudioTier3K:  "3072x3072",
		ImageBillingSize4K: "4096x4096",
	},
	"2:3": {
		ImageBillingSize1K: "1024x1536",
		ImageBillingSize2K: "2048x3072",
		ImageStudioTier3K:  "2160x3240",
		ImageBillingSize4K: "4096x6144",
	},
	"3:2": {
		ImageBillingSize1K: "1536x1024",
		ImageBillingSize2K: "3072x2048",
		ImageStudioTier3K:  "3240x2160",
		ImageBillingSize4K: "6144x4096",
	},
	"9:16": {
		ImageBillingSize1K: "1024x1792",
		ImageBillingSize2K: "2048x3584",
		ImageStudioTier3K:  "1728x3072",
		ImageBillingSize4K: "4096x7168",
	},
	"16:9": {
		ImageBillingSize1K: "1792x1024",
		ImageBillingSize2K: "3584x2048",
		ImageStudioTier3K:  "3072x1728",
		ImageBillingSize4K: "3840x2160",
	},
}

func ListImageStudioCapabilities() ImageStudioCapabilities {
	options := make([]ImageStudioSizeOption, 0, 24)
	for _, aspect := range imageStudioAspectCatalog {
		tiers, ok := imageStudioSizeMatrix[aspect.ID]
		if !ok {
			continue
		}
		for _, tier := range imageStudioTierCatalog {
			size, ok := tiers[tier.ID]
			if !ok || strings.TrimSpace(size) == "" {
				continue
			}
			billingTier, _ := ClassifyImageBillingTier(size)
			if billingTier == "" {
				billingTier = tier.ID
			}
			options = append(options, ImageStudioSizeOption{
				Aspect:      aspect.ID,
				Tier:        tier.ID,
				Size:        size,
				BillingTier: billingTier,
			})
		}
	}
	return ImageStudioCapabilities{
		Aspects:     append([]ImageStudioAspectOption(nil), imageStudioAspectCatalog...),
		Tiers:       append([]ImageStudioTierOption(nil), imageStudioTierCatalog...),
		SizeOptions: options,
	}
}

func ResolveImageStudioSize(aspect, tier, rawSize string) (string, error) {
	rawSize = strings.TrimSpace(rawSize)
	if rawSize != "" {
		if isKnownImageStudioSize(rawSize) {
			return rawSize, nil
		}
		return "", ErrImageStudioSizeNotSupported
	}
	aspect = strings.TrimSpace(aspect)
	tier = strings.TrimSpace(tier)
	switch aspect {
	case "3:4":
		aspect = "2:3"
	case "4:3":
		aspect = "3:2"
	}
	if aspect == "" {
		aspect = "1:1"
	}
	if tier == "" {
		tier = ImageBillingSize1K
	}
	tiers, ok := imageStudioSizeMatrix[aspect]
	if !ok {
		return "", ErrImageStudioSizeNotSupported
	}
	size, ok := tiers[tier]
	if !ok || strings.TrimSpace(size) == "" {
		return "", ErrImageStudioSizeNotSupported
	}
	return size, nil
}

func isKnownImageStudioSize(size string) bool {
	size = strings.TrimSpace(size)
	for _, tiers := range imageStudioSizeMatrix {
		for _, resolved := range tiers {
			if resolved == size {
				return true
			}
		}
	}
	return false
}

func InferImageStudioAspectTier(size string) (aspect, tier string) {
	size = strings.TrimSpace(size)
	for _, aspectOption := range imageStudioAspectCatalog {
		tiers := imageStudioSizeMatrix[aspectOption.ID]
		for _, tierOption := range imageStudioTierCatalog {
			resolved := tiers[tierOption.ID]
			if resolved == size {
				return aspectOption.ID, tierOption.ID
			}
		}
	}
	billingTier, ok := ClassifyImageBillingTier(size)
	if !ok {
		billingTier = ImageBillingSize1K
	}
	return "1:1", billingTier
}

func normalizeStudioImageSize(size string) string {
	if tier, ok := ClassifyImageBillingTier(size); ok {
		return tier
	}
	return ImageBillingSize1K
}
