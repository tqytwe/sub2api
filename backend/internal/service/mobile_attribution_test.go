package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mobileAttributionRepoStub struct {
	writes       []MobileAttributionEventWrite
	binds        []MobileAttributionBinding
	created      bool
	resolved     MobileAttributionReference
	resolveCalls int
	bindErr      error
}

func (r *mobileAttributionRepoStub) RecordEvent(_ context.Context, write MobileAttributionEventWrite) (bool, error) {
	r.writes = append(r.writes, write)
	return r.created, nil
}

func (r *mobileAttributionRepoStub) BindInstallation(_ context.Context, binding MobileAttributionBinding) error {
	r.binds = append(r.binds, binding)
	return r.bindErr
}

func (r *mobileAttributionRepoStub) ResolveReference(_ context.Context, campaignID *int64, referralCode string) (MobileAttributionReference, error) {
	r.resolveCalls++
	result := r.resolved
	result.CampaignID = campaignID
	return result, nil
}

func (r *mobileAttributionRepoStub) ResolveReferralToken(context.Context, string, time.Time) (MobileAttributionReference, error) {
	return r.resolved, nil
}

func (r *mobileAttributionRepoStub) ListInstallations(context.Context, MobileAttributionAdminFilter) ([]MobileAttributionInstallationRow, int64, error) {
	return nil, 0, nil
}

func (r *mobileAttributionRepoStub) Funnel(context.Context, MobileAttributionAdminFilter) ([]MobileAttributionFunnelStep, error) {
	return nil, nil
}

func TestMobileAttributionRecordsAnonymousOpenIdempotently(t *testing.T) {
	repo := &mobileAttributionRepoStub{created: false}
	svc := NewMobileAttributionService(repo, []byte("mobile-attribution-test-key-with-32-bytes"))
	installationID := uuid.NewString()

	result, err := svc.RecordEvent(context.Background(), 0, MobileAttributionEventInput{
		InstallationID: installationID,
		EventType:      "open",
		IdempotencyKey: "app-open-session-1",
		Platform:       "android",
		AppVersion:     "2.0.50",
	})

	require.NoError(t, err)
	require.False(t, result.Created)
	require.Len(t, repo.writes, 1)
	require.Zero(t, repo.writes[0].UserID)
	require.False(t, repo.writes[0].Verified)
	require.NotEmpty(t, repo.writes[0].IdempotencyKeyDigest)
	require.NotEqual(t, "app-open-session-1", string(repo.writes[0].IdempotencyKeyDigest))
	require.Empty(t, repo.binds)
}

func TestMobileAttributionRegisterRequiresAuthenticationAndBinds(t *testing.T) {
	repo := &mobileAttributionRepoStub{created: true}
	svc := NewMobileAttributionService(repo, []byte("mobile-attribution-test-key-with-32-bytes"))
	input := MobileAttributionEventInput{
		InstallationID: uuid.NewString(), EventType: "register", IdempotencyKey: "register-1", Platform: "ios",
	}

	_, err := svc.RecordEvent(context.Background(), 0, input)
	require.ErrorIs(t, err, ErrMobileAttributionAuthenticationRequired)
	require.Empty(t, repo.writes)

	_, err = svc.RecordEvent(context.Background(), 42, input)
	require.NoError(t, err)
	require.Len(t, repo.binds, 1)
	require.Equal(t, int64(42), repo.binds[0].UserID)
	require.False(t, repo.binds[0].RequireExisting)
	require.Equal(t, int64(42), repo.writes[0].UserID)
	require.True(t, repo.writes[0].Verified)
}

func TestMobileAttributionActiveRequiresAuthenticatedBoundInstallation(t *testing.T) {
	repo := &mobileAttributionRepoStub{created: true, bindErr: ErrMobileAttributionInstallationUnbound}
	svc := NewMobileAttributionService(repo, []byte("mobile-attribution-test-key-with-32-bytes"))
	input := MobileAttributionEventInput{
		InstallationID: uuid.NewString(), EventType: "active", IdempotencyKey: "active-event-1", Platform: "android",
	}

	_, err := svc.RecordEvent(context.Background(), 0, input)
	require.ErrorIs(t, err, ErrMobileAttributionAuthenticationRequired)
	require.Empty(t, repo.binds)

	_, err = svc.RecordEvent(context.Background(), 42, input)
	require.ErrorIs(t, err, ErrMobileAttributionInstallationUnbound)
	require.Len(t, repo.binds, 1)
	require.True(t, repo.binds[0].RequireExisting)
	require.Empty(t, repo.writes)
}

func TestMobileAttributionAuthenticatedEventsUseServerTime(t *testing.T) {
	repo := &mobileAttributionRepoStub{created: true}
	svc := NewMobileAttributionService(repo, []byte("mobile-attribution-test-key-with-32-bytes"))
	clientTime := time.Now().UTC().Add(-20 * 24 * time.Hour)
	before := time.Now().UTC()

	_, err := svc.RecordEvent(context.Background(), 42, MobileAttributionEventInput{
		InstallationID: uuid.NewString(), EventType: "login", IdempotencyKey: "login-event-1", Platform: "android", OccurredAt: &clientTime,
	})

	require.NoError(t, err)
	require.WithinDuration(t, before, repo.writes[0].OccurredAt, 2*time.Second)
	require.True(t, repo.writes[0].Verified)
}

func TestMobileAttributionRejectsTamperedTokenAndStoresOnlyVerifiedDigest(t *testing.T) {
	key := []byte("mobile-attribution-test-key-with-32-bytes")
	repo := &mobileAttributionRepoStub{created: true, resolved: MobileAttributionReference{ReferrerUserID: mobileAttributionInt64Ptr(91)}}
	svc := NewMobileAttributionService(repo, key)
	payload := MobileAttributionSignedPayload{CampaignID: mobileAttributionInt64Ptr(7), ReferralCode: "INVITEABC", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	token := signMobileAttributionTestPayload(t, payload, key)

	_, err := svc.RecordEvent(context.Background(), 0, MobileAttributionEventInput{
		InstallationID: uuid.NewString(), EventType: "click", IdempotencyKey: "click-one", Platform: "android", AttributionToken: token + "tampered",
	})
	require.ErrorIs(t, err, ErrMobileAttributionInvalidToken)
	require.Empty(t, repo.writes)

	_, err = svc.RecordEvent(context.Background(), 0, MobileAttributionEventInput{
		InstallationID: uuid.NewString(), EventType: "click", IdempotencyKey: "click-two", Platform: "android", AttributionToken: token,
	})
	require.NoError(t, err)
	require.Equal(t, 1, repo.resolveCalls)
	require.Len(t, repo.writes, 1)
	require.NotEmpty(t, repo.writes[0].AttributionDigest)
	require.NotEqual(t, token, string(repo.writes[0].AttributionDigest))
	require.Equal(t, int64(91), *repo.writes[0].ReferrerUserID)
}

func TestMobileAttributionSanitizesMetadata(t *testing.T) {
	repo := &mobileAttributionRepoStub{created: true}
	svc := NewMobileAttributionService(repo, []byte("mobile-attribution-test-key-with-32-bytes"))
	_, err := svc.RecordEvent(context.Background(), 0, MobileAttributionEventInput{
		InstallationID: uuid.NewString(), EventType: "share", IdempotencyKey: "share-event-1", Platform: "android",
		Metadata: map[string]string{"surface": `home"</script>`, "token": "must-drop", "channel": "wechat", "poster_theme": "celebration"},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"surface": `home"</script>`, "channel": "wechat", "poster_theme": "celebration"}, repo.writes[0].Metadata)
}

func TestMobileAttributionAcceptsServerVerifiedReferralCampaignToken(t *testing.T) {
	campaignID, referrerID := int64(12), int64(44)
	repo := &mobileAttributionRepoStub{created: true, resolved: MobileAttributionReference{ReferralCampaignID: mobileAttributionInt64Ptr(campaignID), ReferrerUserID: mobileAttributionInt64Ptr(referrerID)}}
	svc := NewMobileAttributionService(repo, []byte("mobile-attribution-test-key-with-32-bytes"))
	_, err := svc.RecordEvent(context.Background(), 0, MobileAttributionEventInput{
		InstallationID: uuid.NewString(), EventType: "click", IdempotencyKey: "referral-click-1", Platform: "android",
		AttributionToken: "v1.server-issued.payload",
	})
	require.NoError(t, err)
	require.Nil(t, repo.writes[0].CampaignID)
	require.Equal(t, &campaignID, repo.writes[0].ReferralCampaignID)
	require.Equal(t, &referrerID, repo.writes[0].ReferrerUserID)
	require.NotEmpty(t, repo.writes[0].AttributionDigest)
}

func signMobileAttributionTestPayload(t *testing.T, payload MobileAttributionSignedPayload, key []byte) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("sub2api:mobile-attribution:v1:" + encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func mobileAttributionInt64Ptr(value int64) *int64 { return &value }
