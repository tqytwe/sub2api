package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	MobilePushStatusPending    = "pending"
	MobilePushStatusProcessing = "processing"
	MobilePushStatusSent       = "sent"
	MobilePushStatusFailed     = "failed"
	MobilePushStatusSkipped    = "skipped"
)

var (
	ErrMobileDeviceNotFound = infraerrors.NotFound("MOBILE_DEVICE_NOT_FOUND", "mobile device not found")
	ErrMobileDeviceInvalid  = infraerrors.BadRequest("MOBILE_DEVICE_INVALID", "mobile device registration is invalid")
	ErrMobilePushInvalid    = infraerrors.BadRequest("MOBILE_PUSH_INVALID", "mobile push event is invalid")
	ErrFCMNotConfigured     = errors.New("fcm credentials are not configured")
	ErrFCMTokenInvalid      = errors.New("fcm registration token is permanently invalid")
	ErrMobilePushClaimLost  = errors.New("mobile push outbox claim was lost")
)

type MobileDeviceRegistration struct {
	InstallationID string
	Platform       string
	FCMToken       string
	AppVersion     string
	Locale         string
}

// MobileDevice deliberately excludes both the plaintext token and its stored
// ciphertext from JSON. TokenFingerprint is derived from SHA-256, not the token.
type MobileDevice struct {
	ID               string     `json:"id"`
	UserID           int64      `json:"-"`
	InstallationID   string     `json:"installation_id"`
	Platform         string     `json:"platform"`
	PushProvider     string     `json:"push_provider"`
	TokenCiphertext  string     `json:"-"`
	TokenHash        string     `json:"-"`
	TokenFingerprint string     `json:"token_fingerprint"`
	AppVersion       string     `json:"app_version"`
	Locale           string     `json:"locale"`
	Enabled          bool       `json:"enabled"`
	LastSeenAt       time.Time  `json:"last_seen_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type MobileDeviceWrite struct {
	UserID          int64
	InstallationID  string
	Platform        string
	PushProvider    string
	TokenCiphertext string
	TokenHash       string
	AppVersion      string
	Locale          string
	LastSeenAt      time.Time
}

type MobilePushEvent struct {
	UserID         int64
	IdempotencyKey string
	EventType      string
	SourceType     string
	SourceID       string
	TitleZh        string
	BodyZh         string
	Data           map[string]string
	AvailableAt    time.Time
}

type MobilePushOutboxItem struct {
	ID             int64             `json:"id"`
	UserID         int64             `json:"-"`
	DedupeKeyHash  string            `json:"-"`
	EventType      string            `json:"event_type"`
	SourceType     string            `json:"source_type,omitempty"`
	SourceID       string            `json:"source_id,omitempty"`
	TitleZh        string            `json:"title_zh"`
	BodyZh         string            `json:"body_zh"`
	Data           map[string]string `json:"data"`
	Status         string            `json:"status"`
	Attempts       int               `json:"attempts"`
	LastErrorCode  *string           `json:"last_error_code,omitempty"`
	AvailableAt    time.Time         `json:"available_at"`
	SentAt         *time.Time        `json:"sent_at,omitempty"`
	ClaimToken     string            `json:"-"`
	LeaseExpiresAt *time.Time        `json:"-"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type MobilePushDeviceDelivery struct {
	ID          int64
	OutboxID    int64
	Device      MobileDevice
	Status      string
	Attempts    int
	AvailableAt time.Time
}

type MobilePushDelivery struct {
	EventType  string
	SourceType string
	SourceID   string
	TitleZh    string
	BodyZh     string
	Data       map[string]string
}

type MobilePushRepository interface {
	UpsertDevice(context.Context, MobileDeviceWrite) (*MobileDevice, error)
	RevokeDevice(context.Context, int64, string, time.Time) (bool, error)
	EnqueuePush(context.Context, *MobilePushOutboxItem) (*MobilePushOutboxItem, bool, error)
	ClaimPendingPush(context.Context, time.Time) (*MobilePushOutboxItem, error)
	PreparePushDeliveries(context.Context, int64, int64, string, time.Time) ([]MobilePushDeviceDelivery, error)
	MarkPushDeliverySent(context.Context, int64, int64, string, time.Time) error
	FailPushDelivery(context.Context, int64, int64, string, string, time.Time, bool) error
	RevokeDeviceByID(context.Context, string, time.Time) error
	FinalizePush(context.Context, int64, string, time.Time) error
	MarkPushSkipped(context.Context, int64, string, string, time.Time) error
	RequeuePush(context.Context, int64, string, string, time.Time, bool) error
}

type FCMSender interface {
	Configured() bool
	Send(context.Context, string, MobilePushDelivery) error
}

type MobilePushService struct {
	repo   MobilePushRepository
	cipher SecretEncryptor
	sender FCMSender
	now    func() time.Time
}

func NewMobilePushService(repo MobilePushRepository, cipher SecretEncryptor, sender FCMSender) *MobilePushService {
	return &MobilePushService{repo: repo, cipher: cipher, sender: sender, now: time.Now}
}

func (s *MobilePushService) RegisterDevice(ctx context.Context, userID int64, registration MobileDeviceRegistration) (*MobileDevice, error) {
	if s == nil || s.repo == nil || s.cipher == nil || userID <= 0 {
		return nil, ErrMobileDeviceInvalid
	}
	installationID, err := uuid.Parse(strings.TrimSpace(registration.InstallationID))
	if err != nil {
		return nil, ErrMobileDeviceInvalid
	}
	platform := strings.ToLower(strings.TrimSpace(registration.Platform))
	if platform != "android" && platform != "ios" && platform != "web" {
		return nil, ErrMobileDeviceInvalid
	}
	token := strings.TrimSpace(registration.FCMToken)
	if len(token) < 16 || len(token) > 4096 {
		return nil, ErrMobileDeviceInvalid
	}
	appVersion := strings.TrimSpace(registration.AppVersion)
	locale := strings.TrimSpace(registration.Locale)
	if len(appVersion) > 64 || len(locale) > 32 {
		return nil, ErrMobileDeviceInvalid
	}
	if locale == "" {
		locale = "zh-CN"
	}
	ciphertext, err := s.cipher.Encrypt(token)
	if err != nil {
		return nil, fmt.Errorf("encrypt mobile push token: %w", err)
	}
	tokenHash := mobilePushSHA256(token)
	now := s.now().UTC()
	device, err := s.repo.UpsertDevice(ctx, MobileDeviceWrite{
		UserID: userID, InstallationID: installationID.String(), Platform: platform,
		PushProvider: "fcm", TokenCiphertext: ciphertext, TokenHash: tokenHash,
		AppVersion: appVersion, Locale: locale, LastSeenAt: now,
	})
	if err != nil {
		return nil, err
	}
	if device != nil {
		device.TokenHash = tokenHash
	}
	return mobileDeviceForResponse(device), nil
}

func (s *MobilePushService) DeleteDevice(ctx context.Context, userID int64, installationID string) error {
	if s == nil || s.repo == nil || userID <= 0 {
		return ErrMobileDeviceInvalid
	}
	id, err := uuid.Parse(strings.TrimSpace(installationID))
	if err != nil {
		return ErrMobileDeviceNotFound
	}
	deleted, err := s.repo.RevokeDevice(ctx, userID, id.String(), s.now().UTC())
	if err != nil {
		return err
	}
	if !deleted {
		return ErrMobileDeviceNotFound
	}
	return nil
}

func (s *MobilePushService) Enqueue(ctx context.Context, event MobilePushEvent) (*MobilePushOutboxItem, bool, error) {
	if s == nil || s.repo == nil {
		return nil, false, ErrMobilePushInvalid
	}
	normalized, err := normalizeMobilePushEvent(event, s.now().UTC())
	if err != nil {
		return nil, false, err
	}
	normalizedHash, err := mobilePushDedupeHash(normalized)
	if err != nil {
		return nil, false, err
	}
	item := &MobilePushOutboxItem{
		UserID: normalized.UserID, DedupeKeyHash: normalizedHash,
		EventType: normalized.EventType, SourceType: normalized.SourceType, SourceID: normalized.SourceID,
		TitleZh: normalized.TitleZh, BodyZh: normalized.BodyZh, Data: normalized.Data,
		Status: MobilePushStatusPending, AvailableAt: normalized.AvailableAt,
	}
	return s.repo.EnqueuePush(ctx, item)
}

// DispatchOne returns without claiming an item when FCM is unavailable. This
// keeps the durable queue pending and prevents push configuration from failing
// the business operation that created the event.
func (s *MobilePushService) DispatchOne(ctx context.Context) (bool, error) {
	if s == nil || s.repo == nil || s.sender == nil || !s.sender.Configured() {
		return false, nil
	}
	item, err := s.repo.ClaimPendingPush(ctx, s.now().UTC())
	if err != nil || item == nil {
		return false, err
	}
	deliveries, err := s.repo.PreparePushDeliveries(ctx, item.ID, item.UserID, item.ClaimToken, s.now().UTC())
	if err != nil {
		_ = s.repo.RequeuePush(ctx, item.ID, item.ClaimToken, "PREPARE_FAILED", s.now().UTC().Add(time.Minute), true)
		return true, err
	}
	if len(deliveries) == 0 {
		return true, s.repo.FinalizePush(ctx, item.ID, item.ClaimToken, s.now().UTC())
	}
	delivery := MobilePushDelivery{EventType: item.EventType, SourceType: item.SourceType, SourceID: item.SourceID, TitleZh: item.TitleZh, BodyZh: item.BodyZh, Data: cloneMobilePushData(item.Data)}
	for _, target := range deliveries {
		token, decryptErr := s.cipher.Decrypt(target.Device.TokenCiphertext)
		if decryptErr != nil {
			if err := s.repo.FailPushDelivery(ctx, target.ID, item.ID, item.ClaimToken, "TOKEN_DECRYPT_FAILED", s.now().UTC().Add(time.Minute), false); err != nil {
				return true, err
			}
			continue
		}
		sendErr := s.sender.Send(ctx, token, delivery)
		if errors.Is(sendErr, ErrFCMNotConfigured) {
			return true, s.repo.RequeuePush(ctx, item.ID, item.ClaimToken, "FCM_NOT_CONFIGURED", s.now().UTC(), false)
		}
		if errors.Is(sendErr, ErrFCMTokenInvalid) {
			if err := s.repo.RevokeDeviceByID(ctx, target.Device.ID, s.now().UTC()); err != nil {
				return true, err
			}
			if err := s.repo.FailPushDelivery(ctx, target.ID, item.ID, item.ClaimToken, "TOKEN_INVALID", s.now().UTC(), true); err != nil {
				return true, err
			}
			continue
		}
		if sendErr != nil {
			if err := s.repo.FailPushDelivery(ctx, target.ID, item.ID, item.ClaimToken, "DELIVERY_FAILED", s.now().UTC().Add(time.Minute), false); err != nil {
				return true, err
			}
			continue
		}
		if err := s.repo.MarkPushDeliverySent(ctx, target.ID, item.ID, item.ClaimToken, s.now().UTC()); err != nil {
			return true, err
		}
	}
	return true, s.repo.FinalizePush(ctx, item.ID, item.ClaimToken, s.now().UTC())
}

func mobileDeviceForResponse(device *MobileDevice) *MobileDevice {
	if device == nil {
		return nil
	}
	copy := *device
	copy.TokenFingerprint = mobilePushTokenFingerprint(copy.TokenHash)
	copy.TokenCiphertext = ""
	copy.TokenHash = ""
	return &copy
}

func normalizeMobilePushEvent(event MobilePushEvent, now time.Time) (MobilePushEvent, error) {
	event.EventType = strings.TrimSpace(event.EventType)
	event.SourceType = strings.TrimSpace(event.SourceType)
	event.SourceID = strings.TrimSpace(event.SourceID)
	event.TitleZh = strings.TrimSpace(event.TitleZh)
	event.BodyZh = strings.TrimSpace(event.BodyZh)
	event.IdempotencyKey = strings.TrimSpace(event.IdempotencyKey)
	if event.UserID <= 0 || event.EventType == "" || len(event.EventType) > 64 || event.TitleZh == "" || len([]rune(event.TitleZh)) > 120 || event.BodyZh == "" || len([]rune(event.BodyZh)) > 300 || len(event.SourceType) > 64 || len(event.SourceID) > 128 || len(event.IdempotencyKey) > 128 {
		return event, ErrMobilePushInvalid
	}
	for key, value := range event.Data {
		key = strings.TrimSpace(key)
		if key == "" || len(key) > 64 || len(value) > 512 || mobilePushSensitiveKey(key) {
			return event, ErrMobilePushInvalid
		}
	}
	event.Data = cloneMobilePushData(event.Data)
	if event.AvailableAt.IsZero() {
		event.AvailableAt = now
	}
	return event, nil
}

func mobilePushDedupeHash(event MobilePushEvent) (string, error) {
	if event.IdempotencyKey != "" {
		return mobilePushSHA256(strconv.FormatInt(event.UserID, 10) + "\x00" + event.IdempotencyKey), nil
	}
	keys := make([]string, 0, len(event.Data))
	for key := range event.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	data := make([][2]string, 0, len(keys))
	for _, key := range keys {
		data = append(data, [2]string{key, event.Data[key]})
	}
	payload, err := json.Marshal([]any{event.UserID, event.EventType, event.SourceType, event.SourceID, event.TitleZh, event.BodyZh, data})
	if err != nil {
		return "", err
	}
	return mobilePushSHA256(string(payload)), nil
}

func mobilePushSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	for _, fragment := range []string{"token", "api_key", "apikey", "authorization", "prompt", "chat", "message", "secret", "password"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func mobilePushSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func mobilePushTokenFingerprint(tokenHash string) string {
	if len(tokenHash) < 12 {
		return ""
	}
	return tokenHash[:12]
}

func cloneMobilePushData(data map[string]string) map[string]string {
	copy := make(map[string]string, len(data))
	for key, value := range data {
		copy[key] = value
	}
	return copy
}
