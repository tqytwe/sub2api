package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mobilePushTestCipher struct {
	encryptedPlaintext string
}

func (c *mobilePushTestCipher) Encrypt(plaintext string) (string, error) {
	c.encryptedPlaintext = plaintext
	return "cipher:" + plaintext, nil
}

func (c *mobilePushTestCipher) Decrypt(ciphertext string) (string, error) {
	if len(ciphertext) < len("cipher:") || ciphertext[:len("cipher:")] != "cipher:" {
		return "", errors.New("bad ciphertext")
	}
	return ciphertext[len("cipher:"):], nil
}

type mobilePushTestRepo struct {
	deviceWrite      *MobileDeviceWrite
	revokeUserID     int64
	revokeInstallID  string
	revokeResult     bool
	outboxByHash     map[string]*MobilePushOutboxItem
	claimCount       int
	claimed          *MobilePushOutboxItem
	deliveries       []MobilePushDeviceDelivery
	sentDeliveries   map[int64]bool
	failedDeliveries map[int64]bool
	revokedDeviceID  string
	finalizedID      int64
	markedSkippedID  int64
	requeuedID       int64
	requeueCounted   bool
}

func (r *mobilePushTestRepo) UpsertDevice(_ context.Context, write MobileDeviceWrite) (*MobileDevice, error) {
	r.deviceWrite = &write
	return &MobileDevice{
		ID: uuid.NewString(), UserID: write.UserID, InstallationID: write.InstallationID,
		Platform: write.Platform, PushProvider: write.PushProvider,
		TokenCiphertext: write.TokenCiphertext, TokenHash: write.TokenHash,
		AppVersion: write.AppVersion, Locale: write.Locale, Enabled: true,
		LastSeenAt: write.LastSeenAt, CreatedAt: write.LastSeenAt, UpdatedAt: write.LastSeenAt,
	}, nil
}

func (r *mobilePushTestRepo) RevokeDevice(_ context.Context, userID int64, installationID string, _ time.Time) (bool, error) {
	r.revokeUserID = userID
	r.revokeInstallID = installationID
	return r.revokeResult, nil
}

func (r *mobilePushTestRepo) EnqueuePush(_ context.Context, item *MobilePushOutboxItem) (*MobilePushOutboxItem, bool, error) {
	if r.outboxByHash == nil {
		r.outboxByHash = map[string]*MobilePushOutboxItem{}
	}
	if existing := r.outboxByHash[item.DedupeKeyHash]; existing != nil {
		return existing, false, nil
	}
	copy := *item
	copy.ID = int64(len(r.outboxByHash) + 1)
	r.outboxByHash[item.DedupeKeyHash] = &copy
	return &copy, true, nil
}

func (r *mobilePushTestRepo) ClaimPendingPush(context.Context, time.Time) (*MobilePushOutboxItem, error) {
	r.claimCount++
	return r.claimed, nil
}

func (r *mobilePushTestRepo) PreparePushDeliveries(_ context.Context, _, _ int64, _ string, _ time.Time) ([]MobilePushDeviceDelivery, error) {
	result := make([]MobilePushDeviceDelivery, 0, len(r.deliveries))
	for _, delivery := range r.deliveries {
		if r.sentDeliveries[delivery.ID] || r.failedDeliveries[delivery.ID] {
			continue
		}
		result = append(result, delivery)
	}
	return result, nil
}

func (r *mobilePushTestRepo) MarkPushDeliverySent(_ context.Context, id, _ int64, _ string, _ time.Time) error {
	if r.sentDeliveries == nil {
		r.sentDeliveries = map[int64]bool{}
	}
	r.sentDeliveries[id] = true
	return nil
}

func (r *mobilePushTestRepo) FailPushDelivery(_ context.Context, id, _ int64, _, _ string, _ time.Time, terminal bool) error {
	if terminal {
		if r.failedDeliveries == nil {
			r.failedDeliveries = map[int64]bool{}
		}
		r.failedDeliveries[id] = true
	}
	return nil
}

func (r *mobilePushTestRepo) RevokeDeviceByID(_ context.Context, id string, _ time.Time) error {
	r.revokedDeviceID = id
	return nil
}

func (r *mobilePushTestRepo) FinalizePush(_ context.Context, id int64, _ string, _ time.Time) error {
	r.finalizedID = id
	return nil
}

func (r *mobilePushTestRepo) MarkPushSkipped(_ context.Context, id int64, _, _ string, _ time.Time) error {
	r.markedSkippedID = id
	return nil
}

func (r *mobilePushTestRepo) RequeuePush(_ context.Context, id int64, _, _ string, _ time.Time, countAttempt bool) error {
	r.requeuedID = id
	r.requeueCounted = countAttempt
	return nil
}

type mobilePushTestSender struct {
	configured  bool
	tokens      []string
	sendErr     error
	tokenErrors map[string][]error
}

func (s *mobilePushTestSender) Configured() bool { return s.configured }
func (s *mobilePushTestSender) Send(_ context.Context, token string, _ MobilePushDelivery) error {
	s.tokens = append(s.tokens, token)
	if errorsForToken := s.tokenErrors[token]; len(errorsForToken) > 0 {
		err := errorsForToken[0]
		s.tokenErrors[token] = errorsForToken[1:]
		return err
	}
	return s.sendErr
}

func TestMobilePushServiceRegistersEncryptedDeviceWithoutExposingToken(t *testing.T) {
	repo := &mobilePushTestRepo{}
	cipher := &mobilePushTestCipher{}
	svc := NewMobilePushService(repo, cipher, nil)
	fixedNow := time.Date(2026, time.July, 26, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }
	installationID := uuid.NewString()
	token := "fcm-registration-token-private-value"

	device, err := svc.RegisterDevice(context.Background(), 42, MobileDeviceRegistration{
		InstallationID: installationID,
		Platform:       "android",
		FCMToken:       token,
		AppVersion:     "2.0.34",
		Locale:         "zh-CN",
	})

	require.NoError(t, err)
	require.Equal(t, token, cipher.encryptedPlaintext)
	require.NotNil(t, repo.deviceWrite)
	require.Equal(t, int64(42), repo.deviceWrite.UserID)
	require.Equal(t, "cipher:"+token, repo.deviceWrite.TokenCiphertext)
	require.Equal(t, mobilePushSHA256(token), repo.deviceWrite.TokenHash)
	require.Equal(t, fixedNow, repo.deviceWrite.LastSeenAt)
	require.Equal(t, mobilePushTokenFingerprint(mobilePushSHA256(token)), device.TokenFingerprint)

	raw, err := json.Marshal(device)
	require.NoError(t, err)
	require.NotContains(t, string(raw), token)
	require.NotContains(t, string(raw), "cipher:")
	require.NotContains(t, string(raw), mobilePushSHA256(token))
}

func TestMobilePushServiceDeleteDevicePreservesUserIsolation(t *testing.T) {
	repo := &mobilePushTestRepo{revokeResult: true}
	svc := NewMobilePushService(repo, &mobilePushTestCipher{}, nil)
	installationID := uuid.NewString()

	require.NoError(t, svc.DeleteDevice(context.Background(), 77, installationID))
	require.Equal(t, int64(77), repo.revokeUserID)
	require.Equal(t, installationID, repo.revokeInstallID)

	repo.revokeResult = false
	err := svc.DeleteDevice(context.Background(), 88, installationID)
	require.ErrorIs(t, err, ErrMobileDeviceNotFound)
	require.Equal(t, int64(88), repo.revokeUserID)
}

func TestMobilePushServiceOutboxIsIdempotentAndRejectsSensitiveData(t *testing.T) {
	repo := &mobilePushTestRepo{}
	svc := NewMobilePushService(repo, &mobilePushTestCipher{}, nil)
	event := MobilePushEvent{
		UserID: 9, IdempotencyKey: "image-task-123-completed", EventType: "task.completed",
		SourceType: "image_task", SourceID: "123", TitleZh: "图片生成完成",
		BodyZh: "你的图片已经生成完成", Data: map[string]string{"task_id": "123"},
	}

	first, created, err := svc.Enqueue(context.Background(), event)
	require.NoError(t, err)
	require.True(t, created)
	second, created, err := svc.Enqueue(context.Background(), event)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.ID, second.ID)
	require.Len(t, repo.outboxByHash, 1)

	event.IdempotencyKey = "unsafe"
	event.Data["access_token"] = "must-not-be-stored"
	_, _, err = svc.Enqueue(context.Background(), event)
	require.ErrorIs(t, err, ErrMobilePushInvalid)
	require.Len(t, repo.outboxByHash, 1)
}

func TestMobilePushServiceWithoutFCMCredentialsKeepsQueuePending(t *testing.T) {
	repo := &mobilePushTestRepo{}
	sender := &mobilePushTestSender{configured: false}
	svc := NewMobilePushService(repo, &mobilePushTestCipher{}, sender)

	_, created, err := svc.Enqueue(context.Background(), MobilePushEvent{
		UserID: 10, IdempotencyKey: "support-reply-1", EventType: "support.reply",
		TitleZh: "客服已回复", BodyZh: "你的工单有新回复", Data: map[string]string{"ticket_id": "1"},
	})
	require.NoError(t, err)
	require.True(t, created)

	processed, err := svc.DispatchOne(context.Background())
	require.NoError(t, err)
	require.False(t, processed)
	require.Zero(t, repo.claimCount)
	require.Zero(t, repo.finalizedID)
	require.Zero(t, repo.markedSkippedID)
	require.Zero(t, repo.requeuedID)
}

func TestMobilePushServiceDispatchesDecryptedToken(t *testing.T) {
	deviceID := uuid.NewString()
	repo := &mobilePushTestRepo{
		claimed:    &MobilePushOutboxItem{ID: 31, UserID: 5, ClaimToken: uuid.NewString(), EventType: "task.completed", TitleZh: "任务完成", BodyZh: "任务已经完成"},
		deliveries: []MobilePushDeviceDelivery{{ID: 71, OutboxID: 31, Device: MobileDevice{ID: deviceID, UserID: 5, TokenCiphertext: "cipher:fcm-private-token"}}},
	}
	sender := &mobilePushTestSender{configured: true}
	svc := NewMobilePushService(repo, &mobilePushTestCipher{}, sender)

	processed, err := svc.DispatchOne(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, []string{"fcm-private-token"}, sender.tokens)
	require.True(t, repo.sentDeliveries[71])
	require.Equal(t, int64(31), repo.finalizedID)
}

func TestMobilePushServiceRequeuesWithoutAttemptWhenFCMCredentialsDisappear(t *testing.T) {
	repo := &mobilePushTestRepo{
		claimed:    &MobilePushOutboxItem{ID: 41, UserID: 5, ClaimToken: uuid.NewString(), EventType: "task.completed", TitleZh: "任务完成", BodyZh: "任务已经完成"},
		deliveries: []MobilePushDeviceDelivery{{ID: 81, OutboxID: 41, Device: MobileDevice{ID: uuid.NewString(), UserID: 5, TokenCiphertext: "cipher:fcm-private-token"}}},
	}
	sender := &mobilePushTestSender{configured: true, sendErr: ErrFCMNotConfigured}
	svc := NewMobilePushService(repo, &mobilePushTestCipher{}, sender)

	processed, err := svc.DispatchOne(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, int64(41), repo.requeuedID)
	require.False(t, repo.requeueCounted)
	require.Zero(t, repo.finalizedID)
}

func TestMobilePushServiceRetriesOnlyFailedDevice(t *testing.T) {
	claimToken := uuid.NewString()
	repo := &mobilePushTestRepo{
		claimed: &MobilePushOutboxItem{ID: 51, UserID: 5, ClaimToken: claimToken, EventType: "task.completed", TitleZh: "任务完成", BodyZh: "任务已经完成"},
		deliveries: []MobilePushDeviceDelivery{
			{ID: 91, OutboxID: 51, Device: MobileDevice{ID: uuid.NewString(), UserID: 5, TokenCiphertext: "cipher:token-a"}},
			{ID: 92, OutboxID: 51, Device: MobileDevice{ID: uuid.NewString(), UserID: 5, TokenCiphertext: "cipher:token-b"}},
		},
	}
	sender := &mobilePushTestSender{configured: true, tokenErrors: map[string][]error{"token-b": {errors.New("temporary")}}}
	svc := NewMobilePushService(repo, &mobilePushTestCipher{}, sender)

	processed, err := svc.DispatchOne(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	processed, err = svc.DispatchOne(context.Background())
	require.NoError(t, err)
	require.True(t, processed)

	require.Equal(t, []string{"token-a", "token-b", "token-b"}, sender.tokens)
	require.True(t, repo.sentDeliveries[91])
	require.True(t, repo.sentDeliveries[92])
}

func TestMobilePushServiceRevokesPermanentlyInvalidToken(t *testing.T) {
	deviceID := uuid.NewString()
	repo := &mobilePushTestRepo{
		claimed:    &MobilePushOutboxItem{ID: 61, UserID: 5, ClaimToken: uuid.NewString(), EventType: "task.completed", TitleZh: "任务完成", BodyZh: "任务已经完成"},
		deliveries: []MobilePushDeviceDelivery{{ID: 101, OutboxID: 61, Device: MobileDevice{ID: deviceID, UserID: 5, TokenCiphertext: "cipher:invalid-token"}}},
	}
	sender := &mobilePushTestSender{configured: true, tokenErrors: map[string][]error{"invalid-token": {ErrFCMTokenInvalid}}}
	svc := NewMobilePushService(repo, &mobilePushTestCipher{}, sender)

	processed, err := svc.DispatchOne(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, deviceID, repo.revokedDeviceID)
	require.True(t, repo.failedDeliveries[101])
	require.Equal(t, int64(61), repo.finalizedID)
}
