package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const (
	LiveControllerPending  = "pending"
	LiveControllerObserver = "observer"
	LiveControllerProxy    = "proxy"
	LiveControllerClosed   = "closed"
)

var (
	ErrLiveUnavailable       = errors.New("live is unavailable")
	ErrLiveConcurrencyFull   = errors.New("live concurrency is full")
	ErrLiveCallNotFound      = errors.New("live call not found")
	ErrLiveIdentityMismatch  = errors.New("live call identity mismatch")
	ErrLiveControllerChanged = errors.New("live controller changed")
	// ErrLiveUsagePersistence is terminal for a live sideband. Continuing after
	// receiving billable usage that cannot be durably recorded would make the
	// session appear successful while silently losing a charge.
	ErrLiveUsagePersistence = errors.New("live usage persistence failed")
)

type LiveAttestationUnavailableError struct {
	Reason string
}

func (e *LiveAttestationUnavailableError) Error() string {
	if e == nil || e.Reason == "" {
		return "Live attestation is unavailable"
	}
	return "Live attestation is unavailable: " + e.Reason
}

// LiveCallRequest 是两个下游创建协议归一后的请求。Session 不做结构改写。
type LiveCallRequest struct {
	SDP     string          `json:"sdp"`
	Session json.RawMessage `json:"session"`
}

type LiveCallIdentity struct {
	APIKeyID        int64
	UserID          int64
	GroupID         *int64
	SubscriptionID  *int64
	UserAgent       string
	IPAddress       string
	InboundEndpoint string
}

// liveAPIKeyLoader is deliberately smaller than APIKeyRepository. Live only
// needs the already-authorized key's current billing relationship at settlement
// time; it must never read or retain the credential material itself.
type liveAPIKeyLoader interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
}

type LiveCallRecord struct {
	CallID          string
	CallHash        string
	AccountID       int64
	APIKeyID        int64
	UserID          int64
	GroupID         int64
	SubscriptionID  int64
	LeaseID         string
	Model           string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	Controller      string
	ControllerOwner string
	UserAgent       string
	IPAddress       string
	InboundEndpoint string
	// Usage is the additive, de-duplicated total from upstream response.done
	// events. It deliberately contains no transcript, audio, or raw response ID.
	Usage OpenAIUsage
	// AttestationCiphertext 仅用于让同一会话的 Sideband 复用创建时的证明。
	AttestationCiphertext string
}

type LiveCallCreated struct {
	SDP      []byte
	CallID   string
	Location string
	Account  *Account
}

// LiveCallStore 由 GatewayCache 的 Redis 实现可选提供，避免扩大旧缓存接口。
type LiveCallStore interface {
	SaveLiveCall(ctx context.Context, record *LiveCallRecord, ttl time.Duration) error
	GetLiveCall(ctx context.Context, callHash string) (*LiveCallRecord, error)
	ClaimLiveController(ctx context.Context, callHash, controller, owner string) (bool, error)
	ReleaseLiveController(ctx context.Context, callHash, owner string) (bool, error)
	GetLiveController(ctx context.Context, callHash string) (string, error)
	// AccumulateLiveUsage atomically adds one response.done usage payload. The
	// response ID is used only to reject replayed terminal events after a
	// sideband reconnect; implementations must not retain it in plaintext.
	AccumulateLiveUsage(ctx context.Context, callHash, responseID string, usage OpenAIUsage) (added bool, err error)
	MarkLiveCallClosed(ctx context.Context, callHash string, ttl time.Duration) (bool, error)
}

const (
	// LiveSettlementClosing is a durable finalization intent. It is not
	// billable until Redis confirms the call was closed, or the original live
	// session has reached its hard expiry.
	LiveSettlementClosing = "closing"
	LiveSettlementReady   = "ready"
)

// LiveSettlementJob deliberately persists only the billing snapshot required
// to replay RecordUsage after a process crash. It never stores a transcript,
// audio frames, upstream response IDs, credentials, or attestation material.
type LiveSettlementJob struct {
	ID          int64
	CallHash    string
	Record      *LiveCallRecord
	Status      string
	Attempts    int
	AvailableAt time.Time
	ClaimedBy   string
}

// LiveSettlementOutboxRepository is a durable PostgreSQL boundary. Redis is
// still the live-control source of truth, but it must not be the sole place a
// closed call's billable usage exists.
type LiveSettlementOutboxRepository interface {
	Enqueue(ctx context.Context, record *LiveCallRecord) error
	Activate(ctx context.Context, callHash string) error
	ListClosing(ctx context.Context, limit int) ([]LiveSettlementJob, error)
	Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]LiveSettlementJob, error)
	Ack(ctx context.Context, id int64, workerID string) error
	Retry(ctx context.Context, id int64, workerID string, availableAt time.Time, lastError string) error
}

func addLiveUsage(left, right OpenAIUsage) OpenAIUsage {
	return OpenAIUsage{
		InputTokens:                   left.InputTokens + right.InputTokens,
		InputAudioTokens:              left.InputAudioTokens + right.InputAudioTokens,
		ImageInputTokens:              left.ImageInputTokens + right.ImageInputTokens,
		OutputTokens:                  left.OutputTokens + right.OutputTokens,
		OutputAudioTokens:             left.OutputAudioTokens + right.OutputAudioTokens,
		CacheCreationInputTokens:      left.CacheCreationInputTokens + right.CacheCreationInputTokens,
		CacheCreationInputAudioTokens: left.CacheCreationInputAudioTokens + right.CacheCreationInputAudioTokens,
		CacheReadInputTokens:          left.CacheReadInputTokens + right.CacheReadInputTokens,
		CacheReadInputAudioTokens:     left.CacheReadInputAudioTokens + right.CacheReadInputAudioTokens,
		ImageOutputTokens:             left.ImageOutputTokens + right.ImageOutputTokens,
	}
}

type LiveConcurrencyCache interface {
	AcquireLiveLease(
		ctx context.Context,
		accountID int64,
		accountMax int,
		userID int64,
		userMax int,
		apiKeyID int64,
		leaseID string,
		replacingRegularSlots bool,
	) (bool, error)
	RefreshLiveLease(ctx context.Context, accountID, userID, apiKeyID int64, leaseID string) (bool, error)
	ReleaseLiveLease(ctx context.Context, accountID, userID, apiKeyID int64, leaseID string) error
}
