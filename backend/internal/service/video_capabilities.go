package service

import "strings"

// VideoModelCapabilities is the server-owned mobile contract. Clients must
// not infer support from a model or group display name.
type VideoModelCapabilities struct {
	Operations           []string `json:"operations"`
	SupportedResolutions []string `json:"supported_resolutions"`
	SupportedRatios      []string `json:"supported_ratios"`
	SupportedDurations   []int    `json:"supported_durations"`
	MaxReferenceImages   int      `json:"max_reference_images"`
	MaxReferenceVideos   int      `json:"max_reference_videos"`
	MaxReferenceAudios   int      `json:"max_reference_audios"`
	GenerateAudio        bool     `json:"generate_audio"`
	Watermark            bool     `json:"watermark"`
}

func (c VideoModelCapabilities) SupportsResolution(value string) bool {
	normalized, ok := LookupVideoBillingResolution(value)
	if !ok {
		return false
	}
	for _, candidate := range c.SupportedResolutions {
		if normalized == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func (c VideoModelCapabilities) SupportsDuration(value int) bool {
	for _, candidate := range c.SupportedDurations {
		if candidate == value {
			return true
		}
	}
	return false
}

// ResolveVideoModelCapabilities requires a priced model in the authorized
// group. A display name or provider model list alone cannot grant capability.
func ResolveVideoModelCapabilities(group Group, platform, model string) (VideoModelCapabilities, bool) {
	model = strings.TrimSpace(model)
	platform = strings.ToLower(strings.TrimSpace(platform))
	if model == "" || !group.HasVideoGenerationCapability() || !groupHasAnyVideoPrice(&group, model) {
		return VideoModelCapabilities{}, false
	}
	resolutions := make([]string, 0, 3)
	for _, resolution := range []string{VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P} {
		if price := group.GetVideoPriceForModel(model, resolution); validVideoUnitPrice(price) {
			resolutions = append(resolutions, resolution)
		}
	}
	capability := VideoModelCapabilities{
		Operations:           []string{"text_to_video"},
		SupportedResolutions: resolutions,
		SupportedRatios:      []string{"16:9", "9:16", "1:1"},
		SupportedDurations:   []int{5, 8, 10, 15},
		MaxReferenceImages:   1,
		MaxReferenceVideos:   0,
		MaxReferenceAudios:   0,
		GenerateAudio:        false,
		Watermark:            false,
	}
	// Seedance uses a different task endpoint and accepts richer structured
	// reference content than the OpenAI-compatible Agnes path. Keep this model
	// family explicit so a generic OpenAI group never accidentally receives its
	// larger reference limits or optional controls.
	if IsSeedanceVideoModel(model) && (platform == PlatformOpenAI || platform == PlatformComposite) {
		capability.Operations = []string{"text_to_video", "image_to_video", "video_reference", "audio_reference"}
		capability.SupportedRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"}
		// -1 is the upstream "smart duration" sentinel. It is deliberately
		// advertised only for Seedance; the request validator and billing path
		// normalize its estimate to the documented eight-second default.
		capability.SupportedDurations = []int{-1, 4, 5, 6, 8, 10, 12, 15}
		capability.MaxReferenceImages = 9
		capability.MaxReferenceVideos = 3
		capability.MaxReferenceAudios = 3
		capability.GenerateAudio = true
		capability.Watermark = true
		return capability, true
	}
	switch platform {
	case PlatformGrok:
		capability.Operations = []string{"text_to_video", "image_to_video", "video_reference"}
		capability.MaxReferenceVideos = 1
	case PlatformOpenAI, PlatformComposite:
		capability.Operations = []string{"text_to_video", "image_to_video"}
	default:
		return VideoModelCapabilities{}, false
	}
	return capability, true
}

// IsSeedanceVideoModel is intentionally based on the public model name. The
// current account model mapper resolves that name to the Ark-compatible
// upstream after authorization, while the mobile client never receives an
// upstream URL or credential.
func IsSeedanceVideoModel(model string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(model)), "seedance")
}

func groupHasAnyVideoPrice(group *Group, model string) bool {
	if group == nil {
		return false
	}
	for _, resolution := range []string{VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P} {
		if group.GetVideoPriceForModel(model, resolution) != nil {
			return true
		}
	}
	return false
}
