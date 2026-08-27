package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// MobileVideoJobCreateInput is the client-facing, provider-agnostic request
// accepted by the mobile video BFF. Provider credentials, upstream URLs and
// prices are deliberately absent from this type.
type MobileVideoJobCreateInput struct {
	GroupID int64
	// ExecutionAPIKeyID is assigned by the server after it has authorized the
	// requested group. It is never accepted from the mobile JSON payload.
	ExecutionAPIKeyID int64
	Model             string
	Prompt            string
	Resolution        string
	Ratio             string
	DurationSeconds   int
	GenerateAudio     bool
	Watermark         bool
	ReferenceAssetIDs []string
	ClientRequestID   string
}

var (
	ErrMobileVideoGroupRequired           = errors.New("mobile video group is required")
	ErrMobileVideoModelRequired           = errors.New("mobile video model is required")
	ErrMobileVideoPromptRequired          = errors.New("mobile video prompt is required")
	ErrMobileVideoResolutionInvalid       = errors.New("mobile video resolution is invalid")
	ErrMobileVideoDurationInvalid         = errors.New("mobile video duration is invalid")
	ErrMobileVideoClientRequestIDRequired = errors.New("mobile video client request id is required")
	ErrMobileVideoRatioInvalid            = errors.New("mobile video ratio is invalid")
	ErrMobileVideoReferenceInvalid        = errors.New("mobile video reference asset is invalid")
	ErrMobileVideoCapabilityUnavailable   = errors.New("mobile video capability is unavailable")
	ErrMobileVideoModelUnavailable        = errors.New("mobile video model is unavailable")
)

const (
	MobileVideoOperationGenerate = "video.generate"
	MobileVideoOperationRetry    = "video.retry"
)

// MobileVideoCapabilities is a server-authored allow-list for the controls
// shown by a mobile client. Empty slices mean the option is not available.
type MobileVideoCapabilities struct {
	TextToVideo        bool     `json:"text_to_video"`
	ImageToVideo       bool     `json:"image_to_video"`
	VideoReference     bool     `json:"video_reference"`
	AudioReference     bool     `json:"audio_reference"`
	Resolutions        []string `json:"resolutions"`
	Ratios             []string `json:"ratios"`
	Durations          []int    `json:"durations"`
	MaxReferenceImages int      `json:"max_reference_images"`
	MaxReferenceVideos int      `json:"max_reference_videos"`
	MaxReferenceAudios int      `json:"max_reference_audios"`
	GenerateAudio      bool     `json:"generate_audio"`
	Watermark          bool     `json:"watermark"`
}

// DefaultMobileVideoCapabilities is intentionally conservative. Individual
// groups may expose a smaller set, but never a larger set without an explicit
// server-side configuration change.
var DefaultMobileVideoCapabilities = MobileVideoCapabilities{
	TextToVideo:        true,
	ImageToVideo:       true,
	Resolutions:        []string{VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P},
	Ratios:             []string{"16:9", "9:16", "1:1"},
	Durations:          []int{5, 8, 10, 15},
	MaxReferenceImages: 1,
	MaxReferenceVideos: 0,
	MaxReferenceAudios: 0,
	GenerateAudio:      false,
	Watermark:          false,
}

func (c MobileVideoCapabilities) Clone() MobileVideoCapabilities {
	c.Resolutions = append([]string(nil), c.Resolutions...)
	c.Ratios = append([]string(nil), c.Ratios...)
	c.Durations = append([]int(nil), c.Durations...)
	return c
}

func (c MobileVideoCapabilities) HasResolution(value string) bool {
	normalized, ok := LookupVideoBillingResolution(value)
	if !ok {
		return false
	}
	for _, candidate := range c.Resolutions {
		if normalized == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func (c MobileVideoCapabilities) HasDuration(value int) bool {
	for _, candidate := range c.Durations {
		if candidate == value {
			return true
		}
	}
	return false
}

func ValidateMobileVideoJobCreateInput(input MobileVideoJobCreateInput) error {
	if input.GroupID <= 0 {
		return ErrMobileVideoGroupRequired
	}
	if strings.TrimSpace(input.Model) == "" {
		return ErrMobileVideoModelRequired
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return ErrMobileVideoPromptRequired
	}
	if _, ok := LookupVideoBillingResolution(input.Resolution); !ok {
		return ErrMobileVideoResolutionInvalid
	}
	// -1 is Seedance's smart-duration sentinel. Model capability validation
	// below remains the authority that prevents other models from using it.
	if input.DurationSeconds != VideoBillingSmartDurationSeconds &&
		(input.DurationSeconds < VideoBillingMinDurationSeconds || input.DurationSeconds > VideoBillingMaxDurationSeconds) {
		return ErrMobileVideoDurationInvalid
	}
	if strings.TrimSpace(input.ClientRequestID) == "" || len(strings.TrimSpace(input.ClientRequestID)) > 128 {
		return ErrMobileVideoClientRequestIDRequired
	}
	if ratio := strings.TrimSpace(input.Ratio); ratio != "" && !isMobileVideoRatio(ratio) {
		return ErrMobileVideoRatioInvalid
	}
	// Seedance accepts up to 9 images, 3 videos, and 3 audio references. The
	// selected model capability applies the per-kind limits later; this bound
	// only protects the generic request parser.
	if len(input.ReferenceAssetIDs) > 15 {
		return ErrMobileVideoReferenceInvalid
	}
	for _, id := range input.ReferenceAssetIDs {
		if strings.TrimSpace(id) == "" || len(strings.TrimSpace(id)) > 128 {
			return ErrMobileVideoReferenceInvalid
		}
	}
	return nil
}

func isMobileVideoRatio(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive":
		return true
	default:
		return false
	}
}

// BuildMobileVideoTaskCreateInput projects the request into the generic task
// protocol. It intentionally excludes prompt, asset IDs, price and provider
// details from the persisted public resource projection.
func BuildMobileVideoTaskCreateInput(input MobileVideoJobCreateInput) (MobileTaskCreateInput, error) {
	if err := ValidateMobileVideoJobCreateInput(input); err != nil {
		return MobileTaskCreateInput{}, err
	}
	resolution, _ := LookupVideoBillingResolution(input.Resolution)
	fingerprintPayload := struct {
		GroupID           int64    `json:"group_id"`
		Model             string   `json:"model"`
		PromptSHA256      string   `json:"prompt_sha256"`
		Resolution        string   `json:"resolution"`
		Ratio             string   `json:"ratio"`
		DurationSeconds   int      `json:"duration_seconds"`
		GenerateAudio     bool     `json:"generate_audio"`
		Watermark         bool     `json:"watermark"`
		ReferenceAssetIDs []string `json:"reference_asset_ids"`
	}{
		GroupID: input.GroupID, Model: strings.TrimSpace(input.Model),
		PromptSHA256: mobileVideoPromptHash(input.Prompt), Resolution: resolution,
		Ratio: strings.TrimSpace(input.Ratio), DurationSeconds: input.DurationSeconds,
		GenerateAudio: input.GenerateAudio, Watermark: input.Watermark,
		ReferenceAssetIDs: append([]string(nil), input.ReferenceAssetIDs...),
	}
	fingerprintJSON, _ := json.Marshal(fingerprintPayload)
	fingerprintDigest := sha256.Sum256(fingerprintJSON)
	resource := &MobileTaskResource{Type: "video_job", ID: hex.EncodeToString(fingerprintDigest[:])}
	// The generic protocol stores only a stable opaque resource identifier. The
	// execution worker must load the private request payload from its own store.
	return MobileTaskCreateInput{
		Kind:            MobileTaskKindVideo,
		Operation:       MobileVideoOperationGenerate,
		ClientRequestID: strings.TrimSpace(input.ClientRequestID),
		Resource:        resource,
	}, nil
}

func mobileVideoPromptHash(prompt string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(prompt)))
	return hex.EncodeToString(digest[:])
}

// HasVideoGenerationCapability fails closed. A group is video-capable only
// when a concrete per-second price has been configured for at least one
// resolution or model family; a display name such as "video视频" is not used.
func (g *Group) HasVideoGenerationCapability() bool {
	if g == nil || g.Status != "" && g.Status != StatusActive {
		return false
	}
	if validVideoUnitPrice(g.VideoPrice480P) || validVideoUnitPrice(g.VideoPrice720P) || validVideoUnitPrice(g.VideoPrice1080P) {
		return true
	}
	for _, tiers := range g.VideoModelPrices {
		for _, price := range tiers {
			if price >= 0 && !math.IsNaN(price) && !math.IsInf(price, 0) {
				return true
			}
		}
	}
	return false
}

func validVideoUnitPrice(value *float64) bool {
	return value != nil && *value >= 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func ValidateMobileVideoCapability(group *Group, model string, capabilities MobileVideoCapabilities, input MobileVideoJobCreateInput) error {
	if group == nil || !group.HasVideoGenerationCapability() {
		return ErrMobileVideoCapabilityUnavailable
	}
	if strings.TrimSpace(model) == "" || strings.TrimSpace(model) != strings.TrimSpace(input.Model) {
		return ErrMobileVideoModelUnavailable
	}
	if !capabilities.HasResolution(input.Resolution) || !capabilities.HasDuration(input.DurationSeconds) {
		return fmt.Errorf("%w: unsupported video parameter", ErrMobileVideoCapabilityUnavailable)
	}
	return nil
}
