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

// MobileVideoJobCreateInput is the private, provider-neutral input accepted
// by the mobile video BFF. The handler assigns ExecutionAPIKeyID and Adapter
// only after the server-owned capability resolver authorizes the group/model.
type MobileVideoJobCreateInput struct {
	GroupID           int64
	ExecutionAPIKeyID int64
	// ExecutionSnapshot and the pricing fields are private durable inputs. They
	// are populated only after the handler completes live authorization and
	// derives the exact balance hold. Older test/legacy callers may omit all of
	// them; such jobs are never produced by the funded mobile-video endpoint.
	ExecutionSnapshot *MobileVideoExecutionSnapshot
	UnitPriceUSD      float64
	RateMultiplier    float64
	HoldAmount        float64
	Adapter           string
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

const (
	MobileVideoOperationGenerate = "video.generate"
	MobileVideoOperationRetry    = "video.retry"

	// These are execution adapters, not model-family heuristics. The catalog
	// declaration chooses one and the gateway provider rejects unknown values.
	MobileVideoAdapterGrok  = "grok_video"
	MobileVideoAdapterAgnes = "agnes_video"

	// AgnesVideoMobileFrameRate is the reviewed V2.0 frame rate for the
	// published mobile presets. The catalog never exposes a combination that
	// needs the worker to guess a frame rate.
	AgnesVideoMobileFrameRate = 24
)

var (
	ErrMobileVideoGroupRequired           = errors.New("mobile video group is required")
	ErrMobileVideoExecutionKeyRequired    = errors.New("mobile video execution key is required")
	ErrMobileVideoAdapterRequired         = errors.New("mobile video execution adapter is required")
	ErrMobileVideoModelRequired           = errors.New("mobile video model is required")
	ErrMobileVideoPromptRequired          = errors.New("mobile video prompt is required")
	ErrMobileVideoResolutionInvalid       = errors.New("mobile video resolution is invalid")
	ErrMobileVideoDurationInvalid         = errors.New("mobile video duration is invalid")
	ErrMobileVideoClientRequestIDRequired = errors.New("mobile video client request id is required")
	ErrMobileVideoRatioInvalid            = errors.New("mobile video ratio is invalid")
	ErrMobileVideoReferenceInvalid        = errors.New("mobile video reference asset is invalid")
	ErrMobileVideoCapabilityUnavailable   = errors.New("mobile video capability is unavailable")
	ErrMobileVideoModelUnavailable        = errors.New("mobile video model is unavailable")
	ErrMobileVideoPurposeInvalid          = errors.New("mobile video purpose must be video")
	ErrMobileVideoRequestTooLarge         = errors.New("mobile video request is too large")
	ErrMobileVideoFundingInvalid          = errors.New("mobile video funding input is invalid")
	ErrMobileVideoQueueLimit              = errors.New("mobile video active queue limit reached")
)

// MobileVideoCapabilities is the transport-safe subset of a declared model
// capability. It deliberately does not infer anything from the model ID.
type MobileVideoCapabilities struct {
	Operations           []string `json:"operations"`
	SupportedResolutions []string `json:"supported_resolutions"`
	SupportedRatios      []string `json:"supported_ratios"`
	SupportedDurations   []int    `json:"supported_durations"`
	MaxReferenceAssets   int      `json:"max_reference_assets,omitempty"`
	MaxReferenceImages   int      `json:"max_reference_images"`
	MaxReferenceVideos   int      `json:"max_reference_videos"`
	MaxReferenceAudios   int      `json:"max_reference_audios"`
	GenerateAudio        bool     `json:"generate_audio"`
	Watermark            bool     `json:"watermark"`
}

// AgnesVideoMobileRequest is the upstream-shaped request subset accepted by
// Agnes Video V2.0. It deliberately has no generic OpenAI video fields.
type AgnesVideoMobileRequest struct {
	Width     int
	Height    int
	NumFrames int
	FrameRate int
}

// ResolveAgnesVideoMobileRequest turns the only published mobile capability
// matrix into exact upstream fields. Larger tiers and additional ratios are
// deliberately rejected until their input-to-output mapping is documented.
func ResolveAgnesVideoMobileRequest(resolution, ratio string, durationSeconds int) (AgnesVideoMobileRequest, bool) {
	if !strings.EqualFold(strings.TrimSpace(resolution), VideoBillingResolution480P) || strings.TrimSpace(ratio) != "16:9" {
		return AgnesVideoMobileRequest{}, false
	}
	frames, ok := agnesVideoMobileFrames(durationSeconds)
	if !ok {
		return AgnesVideoMobileRequest{}, false
	}
	return AgnesVideoMobileRequest{Width: 1024, Height: 576, NumFrames: frames, FrameRate: AgnesVideoMobileFrameRate}, true
}

// AgnesVideoMobileCapabilitiesSupported validates an administrator-declared
// capability before it can become a runnable task. This is intentionally
// narrower than the generic catalog schema because the execution adapter has
// a verified finite request matrix.
func AgnesVideoMobileCapabilitiesSupported(capabilities MobileVideoCapabilities) bool {
	if capabilities.GenerateAudio || capabilities.Watermark ||
		capabilities.MaxReferenceAssets != 0 || capabilities.MaxReferenceImages != 0 ||
		capabilities.MaxReferenceVideos != 0 || capabilities.MaxReferenceAudios != 0 {
		return false
	}
	if !capabilities.SupportsOperation("generate") || len(capabilities.SupportedResolutions) == 0 ||
		len(capabilities.SupportedRatios) == 0 || len(capabilities.SupportedDurations) == 0 {
		return false
	}
	for _, resolution := range capabilities.SupportedResolutions {
		if !strings.EqualFold(strings.TrimSpace(resolution), VideoBillingResolution480P) {
			return false
		}
	}
	for _, ratio := range capabilities.SupportedRatios {
		if strings.TrimSpace(ratio) != "16:9" {
			return false
		}
	}
	for _, duration := range capabilities.SupportedDurations {
		if _, ok := agnesVideoMobileFrames(duration); !ok {
			return false
		}
	}
	return true
}

func agnesVideoMobileFrames(durationSeconds int) (int, bool) {
	// V2.0 uses 8n+1 frames at 24 fps for its published presets.
	switch durationSeconds {
	case 3:
		return 81, true
	case 5:
		return 121, true
	case 10:
		return 241, true
	case 18:
		return 441, true
	default:
		return 0, false
	}
}

func (c MobileVideoCapabilities) Clone() MobileVideoCapabilities {
	c.Operations = append([]string(nil), c.Operations...)
	c.SupportedResolutions = append([]string(nil), c.SupportedResolutions...)
	c.SupportedRatios = append([]string(nil), c.SupportedRatios...)
	c.SupportedDurations = append([]int(nil), c.SupportedDurations...)
	return c
}

func (c MobileVideoCapabilities) SupportsResolution(value string) bool {
	value = strings.TrimSpace(value)
	for _, candidate := range c.SupportedResolutions {
		if strings.EqualFold(strings.TrimSpace(candidate), value) {
			return true
		}
	}
	return false
}

func (c MobileVideoCapabilities) SupportsDuration(value int) bool {
	for _, candidate := range c.SupportedDurations {
		if candidate == value {
			return true
		}
	}
	return false
}

func (c MobileVideoCapabilities) SupportsOperation(value string) bool {
	for _, candidate := range c.Operations {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

// ValidateMobileVideoJobCreateInput performs only generic input checks. The
// caller must use a declared capability resolver for operations, resolutions,
// ratios, durations, billing, scheduling, and execution-adapter support.
func ValidateMobileVideoJobCreateInput(input MobileVideoJobCreateInput) error {
	if input.GroupID <= 0 {
		return ErrMobileVideoGroupRequired
	}
	if input.ExecutionAPIKeyID <= 0 {
		return ErrMobileVideoExecutionKeyRequired
	}
	if adapter := strings.TrimSpace(input.Adapter); adapter == "" || len(adapter) > 100 {
		return ErrMobileVideoAdapterRequired
	}
	if model := strings.TrimSpace(input.Model); model == "" || len(model) > 255 {
		return ErrMobileVideoModelRequired
	}
	if prompt := strings.TrimSpace(input.Prompt); prompt == "" || len(prompt) > 200_000 {
		return ErrMobileVideoPromptRequired
	}
	if resolution := strings.TrimSpace(input.Resolution); resolution == "" || len(resolution) > 64 {
		return ErrMobileVideoResolutionInvalid
	}
	// The declaration owns allowed durations. This bound only protects storage
	// and upstream payloads while allowing future models to exceed legacy 15 s.
	if input.DurationSeconds <= 0 || input.DurationSeconds > 3_600 {
		return ErrMobileVideoDurationInvalid
	}
	if ratio := strings.TrimSpace(input.Ratio); len(ratio) > 32 {
		return ErrMobileVideoRatioInvalid
	}
	if clientRequestID := strings.TrimSpace(input.ClientRequestID); clientRequestID == "" || len(clientRequestID) > 128 {
		return ErrMobileVideoClientRequestIDRequired
	}
	if len(input.ReferenceAssetIDs) > 32 {
		return ErrMobileVideoReferenceInvalid
	}
	seen := make(map[string]struct{}, len(input.ReferenceAssetIDs))
	for _, id := range input.ReferenceAssetIDs {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 128 {
			return ErrMobileVideoReferenceInvalid
		}
		if _, exists := seen[id]; exists {
			return ErrMobileVideoReferenceInvalid
		}
		seen[id] = struct{}{}
	}
	if input.ExecutionSnapshot != nil {
		if !ValidMobileVideoExecutionSnapshot(input.ExecutionSnapshot, input.ExecutionSnapshot.APIKey.UserID, input.GroupID, input.ExecutionAPIKeyID) ||
			math.IsNaN(input.UnitPriceUSD) || math.IsInf(input.UnitPriceUSD, 0) ||
			math.IsNaN(input.RateMultiplier) || math.IsInf(input.RateMultiplier, 0) ||
			math.IsNaN(input.HoldAmount) || math.IsInf(input.HoldAmount, 0) ||
			input.UnitPriceUSD < 0 || input.RateMultiplier < 0 || input.HoldAmount < 0 ||
			QuantizeUsageBillingAmount(input.RateMultiplier) != QuantizeUsageBillingAmount(input.ExecutionSnapshot.EffectiveVideoRateMultiplier) {
			return ErrMobileVideoFundingInvalid
		}
		expectedHold, err := ValidateMobileVideoHoldAmount(input.UnitPriceUSD, input.DurationSeconds, input.RateMultiplier)
		if err != nil || QuantizeUsageBillingAmount(input.HoldAmount) != expectedHold {
			return ErrMobileVideoFundingInvalid
		}
	}
	return nil
}

// BuildMobileVideoTaskCreateInput projects a video request into the public
// task protocol. Prompt text, reference IDs, upstream routing and credentials
// never enter mobile_tasks.
func BuildMobileVideoTaskCreateInput(input MobileVideoJobCreateInput) (MobileTaskCreateInput, error) {
	if err := ValidateMobileVideoJobCreateInput(input); err != nil {
		return MobileTaskCreateInput{}, err
	}
	fingerprintPayload := struct {
		GroupID         int64    `json:"group_id"`
		Model           string   `json:"model"`
		PromptSHA256    string   `json:"prompt_sha256"`
		Resolution      string   `json:"resolution"`
		Ratio           string   `json:"ratio"`
		DurationSeconds int      `json:"duration_seconds"`
		References      []string `json:"reference_ids"`
	}{
		GroupID: input.GroupID, Model: strings.TrimSpace(input.Model),
		PromptSHA256: mobileVideoPromptHash(input.Prompt), Resolution: strings.TrimSpace(input.Resolution),
		Ratio: strings.TrimSpace(input.Ratio), DurationSeconds: input.DurationSeconds,
		References: append([]string(nil), input.ReferenceAssetIDs...),
	}
	payload, err := json.Marshal(fingerprintPayload)
	if err != nil {
		return MobileTaskCreateInput{}, fmt.Errorf("marshal video task fingerprint: %w", err)
	}
	digest := sha256.Sum256(payload)
	return MobileTaskCreateInput{
		Kind:            MobileTaskKindVideo,
		Operation:       MobileVideoOperationGenerate,
		ClientRequestID: strings.TrimSpace(input.ClientRequestID),
		Resource:        &MobileTaskResource{Type: "video_job", ID: hex.EncodeToString(digest[:])},
	}, nil
}

func mobileVideoPromptHash(prompt string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(prompt)))
	return hex.EncodeToString(digest[:])
}
