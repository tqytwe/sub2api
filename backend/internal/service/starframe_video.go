package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	OpenAIEndpointCapabilityStarframe  OpenAIEndpointCapability = "starframe"
	OpenAIEndpointCapabilityVideos     OpenAIEndpointCapability = "videos"
	OpenAIEndpointCapabilityAgnesVideo OpenAIEndpointCapability = "agnes_video"
	AgnesVideoEndpointContent          AgnesVideoEndpoint       = "content"
	starframeTaskTTL                                            = 24 * time.Hour
	starframeSubmissionTTL                                      = 48 * time.Hour
)

type StarframeVideoOwner struct {
	UserID   int64 `json:"user_id"`
	APIKeyID int64 `json:"api_key_id"`
	GroupID  int64 `json:"group_id"`
}

type StarframeVideoTask struct {
	Owner                  StarframeVideoOwner    `json:"owner"`
	LocalID                string                 `json:"local_id"`
	UpstreamID             string                 `json:"upstream_id"`
	AccountID              int64                  `json:"account_id"`
	BaseURL                string                 `json:"base_url"`
	Model                  string                 `json:"model"`
	ClientTaskID           string                 `json:"client_task_id"`
	UpstreamModel          string                 `json:"upstream_model"`
	Duration               int                    `json:"duration"`
	Resolution             string                 `json:"resolution"`
	UpstreamKeyFingerprint string                 `json:"upstream_key_fingerprint"`
	Billing                *StarframeVideoBilling `json:"billing"`
}

type StarframeVideoRequest struct {
	Model        string
	ClientTaskID string
	Duration     int
	Resolution   string
}

type starframeVideoRequestContextKey struct{}

func WithStarframeVideoRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, starframeVideoRequestContextKey{}, true)
}

func IsStarframeVideoRequest(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(starframeVideoRequestContextKey{}).(bool)
	return value
}

func IsStarframeVideoID(id string) bool { return strings.HasPrefix(id, "sfv_") }

func starframeUpstreamClientTaskID(owner StarframeVideoOwner, original string) string {
	seed := fmt.Sprintf("starframe:%d:%d:%d:%s", owner.GroupID, owner.UserID, owner.APIKeyID, original)
	return fmt.Sprintf("sfn_%x", sha256.Sum256([]byte(seed)))
}

func starframeUpstreamKeyFingerprint(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}

func ParseStarframeVideoRequest(body []byte) (StarframeVideoRequest, error) {
	var info StarframeVideoRequest
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return info, fmt.Errorf("StarFrame requires a JSON object")
	}
	if err := validateStarframeUniqueJSON(body); err != nil {
		return info, err
	}
	for _, key := range []string{"model", "prompt", "mode", "client_task_id"} {
		v := gjson.GetBytes(body, key)
		if v.Type != gjson.String || strings.TrimSpace(v.String()) == "" {
			return info, fmt.Errorf("StarFrame requires string %s", key)
		}
	}
	info.Model = gjson.GetBytes(body, "model").String()
	info.ClientTaskID = gjson.GetBytes(body, "client_task_id").String()
	if len(info.ClientTaskID) > 128 {
		return info, fmt.Errorf("StarFrame client_task_id must be at most 128 characters")
	}
	for i := range info.ClientTaskID {
		if !isSafeUpstreamPathSegmentByte(info.ClientTaskID[i]) {
			return info, fmt.Errorf("StarFrame client_task_id permits only letters, digits, underscore, hyphen and dot")
		}
	}
	switch gjson.GetBytes(body, "mode").String() {
	case "references":
		if gjson.GetBytes(body, "frames").Exists() {
			return info, fmt.Errorf("StarFrame references mode cannot include frames")
		}
		if err := validateStarframeReferences(gjson.GetBytes(body, "references")); err != nil {
			return info, err
		}
	case "frames":
		if gjson.GetBytes(body, "references").Exists() {
			return info, fmt.Errorf("StarFrame frames mode cannot include references")
		}
		for _, key := range []string{"frames.first_frame", "frames.last_frame"} {
			if err := validateStarframeMaterialURL(gjson.GetBytes(body, key)); err != nil {
				return info, err
			}
		}
	default:
		return info, fmt.Errorf("StarFrame mode must be references or frames")
	}
	seconds := gjson.GetBytes(body, "duration")
	if seconds.Type != gjson.Number || seconds.Float() < 1 || seconds.Float() > 15 || math.Trunc(seconds.Float()) != seconds.Float() {
		return info, fmt.Errorf("StarFrame gateway first version requires numeric integer duration from 1 to 15 for exact billing")
	}
	info.Duration = int(seconds.Int())
	info.Resolution = gjson.GetBytes(body, "resolution").String()
	switch info.Resolution {
	case VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P:
	default:
		return info, fmt.Errorf("StarFrame gateway first version requires resolution 480p, 720p or 1080p for exact billing")
	}
	return info, nil
}

func (s *OpenAIGatewayService) ValidateStarframeVideoPricing(ctx context.Context, apiKey *APIKey, model, resolution string) error {
	_, err := s.SnapshotStarframeVideoBilling(ctx, apiKey, StarframeVideoRequest{Model: model, Resolution: resolution, Duration: 1})
	return err
}

func buildStarframeVideoURL(base string, endpoint AgnesVideoEndpoint, id string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("StarFrame requires an explicit trusted HTTPS API base without userinfo, query or fragment")
	}
	path := "/v1/videos"
	if endpoint != AgnesVideoEndpointCreate {
		if !isSafeUpstreamPathSegment(id) {
			return "", fmt.Errorf("invalid StarFrame upstream task ID")
		}
		path += "/" + id
		if endpoint == AgnesVideoEndpointContent {
			path += "/content"
		}
	}
	return buildOpenAIEndpointURL(base, path), nil
}

func starframeTaskCacheKey(localID string, owner StarframeVideoOwner) string {
	return fmt.Sprintf("starframe-task:%d:%d:%d:%s", owner.GroupID, owner.UserID, owner.APIKeyID, localID)
}

func (s *OpenAIGatewayService) LoadStarframeVideoTask(ctx context.Context, id string, owner StarframeVideoOwner) (*StarframeVideoTask, error) {
	if s == nil || s.cache == nil || !IsStarframeVideoID(id) || !isSafeUpstreamPathSegment(id) || owner.UserID <= 0 || owner.APIKeyID <= 0 || owner.GroupID <= 0 {
		return nil, fmt.Errorf("StarFrame task binding unavailable or invalid")
	}
	body, err := s.cache.GetGrokVideoPendingBilling(ctx, starframeTaskCacheKey(id, owner))
	if err != nil {
		return nil, err
	}
	var task StarframeVideoTask
	if len(body) == 0 || json.Unmarshal(body, &task) != nil || task.Owner != owner || task.LocalID != id || task.AccountID <= 0 || len(task.UpstreamKeyFingerprint) != 64 || !isSafeUpstreamPathSegment(task.UpstreamID) {
		return nil, fmt.Errorf("StarFrame task binding not found or expired (24 hour retention)")
	}
	return &task, nil
}

// ForwardStarframeVideo never follows response URLs or retries a submitted create.
// Its caller obtains lookup tasks only through the owner-scoped binding above.
func (s *OpenAIGatewayService) ForwardStarframeVideo(ctx context.Context, c *gin.Context, account *Account, endpoint AgnesVideoEndpoint, task *StarframeVideoTask, body []byte, owner StarframeVideoOwner, billing ...*StarframeVideoBilling) (*OpenAIForwardResult, error) {
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityStarframe) {
		return nil, fmt.Errorf("StarFrame requires explicit protocol, capability and API base on an OpenAI API-key account")
	}
	base := strings.TrimSpace(account.GetCredential("base_url"))
	validatedBase, err := s.validateUpstreamBaseURL(base)
	if err != nil {
		return nil, err
	}
	var info StarframeVideoRequest
	upstreamID := ""
	upstreamModel := ""
	if endpoint == AgnesVideoEndpointCreate {
		info, err = ParseStarframeVideoRequest(body)
		if err != nil {
			return nil, err
		}
		if owner.UserID <= 0 || owner.APIKeyID <= 0 || owner.GroupID <= 0 || s.cache == nil {
			return nil, fmt.Errorf("StarFrame submission requires an available owner binding cache")
		}
		if len(billing) != 1 || !validStarframeVideoBilling(info, billing[0]) {
			return nil, fmt.Errorf("StarFrame creation requires a valid explicit billing snapshot")
		}
		upstreamModel = account.GetMappedModel(info.Model)
		body, err = sjson.SetBytes(body, "client_task_id", starframeUpstreamClientTaskID(owner, info.ClientTaskID))
		if err != nil {
			return nil, err
		}
		if upstreamModel != info.Model {
			body, err = sjson.SetBytes(body, "model", upstreamModel)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if task == nil || task.AccountID != account.ID || task.BaseURL != base || task.Owner != owner {
			return nil, fmt.Errorf("StarFrame task account, owner or API base changed")
		}
		upstreamID = task.UpstreamID
	}
	target, err := buildStarframeVideoURL(validatedBase, endpoint, upstreamID)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, fmt.Errorf("StarFrame account API key is missing")
	}
	if endpoint != AgnesVideoEndpointCreate && (task.UpstreamKeyFingerprint == "" || task.UpstreamKeyFingerprint != starframeUpstreamKeyFingerprint(token)) {
		return nil, fmt.Errorf("StarFrame original upstream API key changed or fingerprint is missing")
	}
	if endpoint == AgnesVideoEndpointCreate {
		claimKey := fmt.Sprintf("starframe-submit:%d:%d:%d:%s", owner.GroupID, owner.UserID, owner.APIKeyID, info.ClientTaskID)
		claimed, claimErr := s.cache.ClaimGrokVideoBilled(ctx, claimKey, starframeSubmissionTTL)
		if claimErr != nil {
			return nil, fmt.Errorf("StarFrame submission claim unavailable")
		}
		if !claimed {
			writeAgnesVideoErrorResponse(c, http.StatusConflict, "starframe_submission_already_attempted", "This client_task_id was already submitted or has an unknown outcome (48 hour retention); query the original task or reconcile with the upstream")
			return nil, fmt.Errorf("StarFrame submission already attempted")
		}
	}
	started := time.Now()
	requestCtx := WithHTTPUpstreamRedirectsDisabled(WithStarframeVideoRequest(ctx))
	req, err := http.NewRequestWithContext(requestCtx, endpoint.httpMethod(), target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if endpoint == AgnesVideoEndpointCreate {
		req.Header.Set("Content-Type", "application/json")
	}
	if endpoint == AgnesVideoEndpointContent {
		req.Header.Set("Accept", "video/*, application/octet-stream")
		for _, name := range []string{"Range", "If-Range"} {
			if value := c.GetHeader(name); value != "" {
				req.Header.Set(name, value)
			}
		}
	}
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("StarFrame upstream request failed; submission outcome may be unknown")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, fmt.Errorf("StarFrame upstream redirects are not permitted")
	}
	result := &OpenAIForwardResult{Duration: time.Since(started), ResponseHeaders: resp.Header.Clone()}
	if endpoint == AgnesVideoEndpointContent && resp.StatusCode < 300 {
		return result, writeGrokMediaContentResponse(c, resp)
	}
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		// Preserve native JSON errors without introducing create failover.
		c.Data(resp.StatusCode, "application/json", responseBody)
		return nil, fmt.Errorf("StarFrame upstream status %d", resp.StatusCode)
	}
	if !gjson.ValidBytes(responseBody) || !gjson.ParseBytes(responseBody).IsObject() {
		return nil, fmt.Errorf("StarFrame upstream returned invalid task JSON")
	}
	if endpoint == AgnesVideoEndpointCreate {
		id := gjson.GetBytes(responseBody, "id")
		if id.Type != gjson.String || !isSafeUpstreamPathSegment(id.String()) {
			return nil, fmt.Errorf("StarFrame create response missing valid task ID; submission outcome unknown")
		}
		task = &StarframeVideoTask{Owner: owner, LocalID: "sfv_" + uuid.NewString(), UpstreamID: id.String(), AccountID: account.ID, BaseURL: base, Model: info.Model, ClientTaskID: info.ClientTaskID, UpstreamModel: upstreamModel, Duration: info.Duration, Resolution: info.Resolution, UpstreamKeyFingerprint: starframeUpstreamKeyFingerprint(token), Billing: billing[0]}
		binding, marshalErr := json.Marshal(task)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if err := s.cache.SetGrokVideoPendingBilling(ctx, starframeTaskCacheKey(task.LocalID, owner), binding, starframeTaskTTL); err != nil {
			writeAgnesVideoErrorResponse(c, http.StatusBadGateway, "starframe_task_binding_failed", "Upstream may have accepted the task but local binding failed; do not resubmit with a new client_task_id, reconcile with the upstream")
			return nil, fmt.Errorf("StarFrame task binding failed after submission")
		}
		result.Model, result.BillingModel, result.UpstreamModel = info.Model, info.Model, upstreamModel
		result.ResponseID = task.LocalID
		result.RequestID = fmt.Sprintf("starframe-video:%d:%s", account.ID, task.UpstreamID)
		result.VideoCount, result.VideoDurationSeconds, result.VideoResolution = 1, info.Duration, info.Resolution
		result.StarframeVideoBilling = billing[0]
	} else {
		if id := gjson.GetBytes(responseBody, "id"); id.Exists() && id.String() != task.UpstreamID {
			return nil, fmt.Errorf("StarFrame returned a different task ID")
		}
	}
	responseBody, err = sjson.SetBytes(responseBody, "id", task.LocalID)
	if err != nil {
		return nil, err
	}
	responseBody, err = sjson.SetBytes(responseBody, "client_task_id", task.ClientTaskID)
	if err != nil {
		return nil, err
	}
	if gjson.GetBytes(responseBody, "metadata.url").Exists() {
		responseBody, err = sjson.SetBytes(responseBody, "metadata.url", "/v1/videos/"+task.LocalID+"/content")
		if err != nil {
			return nil, err
		}
	}
	c.Data(resp.StatusCode, "application/json", responseBody)
	return result, nil
}
