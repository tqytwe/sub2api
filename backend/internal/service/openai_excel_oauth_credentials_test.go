package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

type excelOAuthAccountReaderStub struct {
	account *Account
}

func (s *excelOAuthAccountReaderStub) GetAccount(context.Context, int64) (*Account, error) {
	return s.account, nil
}

func (s *excelOAuthAccountReaderStub) GetProxy(context.Context, int64) (*Proxy, error) {
	return nil, nil
}

type excelOAuthCredentialRepoStub struct {
	mu      sync.Mutex
	record  *OpenAIExcelOAuthCredentialRecord
	saves   int32
	updated chan struct{}
}

func (r *excelOAuthCredentialRepoStub) GetExcelOAuthCredentials(context.Context, int64) (*OpenAIExcelOAuthCredentialRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.record == nil {
		return nil, nil
	}
	copy := *r.record
	if r.record.ExpiresAt != nil {
		expires := *r.record.ExpiresAt
		copy.ExpiresAt = &expires
	}
	return &copy, nil
}

func (r *excelOAuthCredentialRepoStub) SaveExcelOAuthCredentials(_ context.Context, accountID, expectedRevision int64, ciphertext string, expiresAt *time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.record == nil && expectedRevision != 0 || r.record != nil && r.record.Revision != expectedRevision {
		return 0, ErrOpenAIExcelOAuthRevisionConflict
	}
	revision := expectedRevision + 1
	now := time.Now().UTC()
	r.record = &OpenAIExcelOAuthCredentialRecord{AccountID: accountID, CredentialsEncrypted: ciphertext, ExpiresAt: expiresAt, Revision: revision, UpdatedAt: now}
	atomic.AddInt32(&r.saves, 1)
	if r.updated != nil {
		select {
		case r.updated <- struct{}{}:
		default:
		}
	}
	return revision, nil
}

type excelOAuthEncryptorStub struct{}

func (excelOAuthEncryptorStub) Encrypt(value string) (string, error) {
	return "encrypted:" + base64.RawStdEncoding.EncodeToString([]byte(value)), nil
}
func (excelOAuthEncryptorStub) Decrypt(value string) (string, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, "encrypted:"))
	return string(decoded), err
}

type excelOAuthClientStub struct {
	refreshCalls int32
	refresh      func() *openai.TokenResponse
}

func (s *excelOAuthClientStub) ExchangeCode(context.Context, string, string, string, string, string) (*openai.TokenResponse, error) {
	return &openai.TokenResponse{AccessToken: "exchange-access", RefreshToken: "exchange-refresh", IDToken: testExcelIDToken(), ExpiresIn: 3600}, nil
}

func (s *excelOAuthClientStub) RefreshToken(context.Context, string, string) (*openai.TokenResponse, error) {
	return nil, errors.New("unexpected legacy refresh")
}

func (s *excelOAuthClientStub) RefreshTokenWithClientID(context.Context, string, string, string) (*openai.TokenResponse, error) {
	atomic.AddInt32(&s.refreshCalls, 1)
	if s.refresh != nil {
		return s.refresh(), nil
	}
	return &openai.TokenResponse{AccessToken: "refreshed-access", RefreshToken: "refreshed-refresh", ExpiresIn: 3600}, nil
}

type excelOAuthAccessStub struct {
	mu          sync.Mutex
	calls       int
	accessToken string
	accountID   string
	proxyURL    string
	body        []byte
}

func (s *excelOAuthAccessStub) FetchAccessModels(_ context.Context, accessToken, accountID, proxyURL string) ([]byte, int, error) {
	s.mu.Lock()
	s.calls++
	s.accessToken = accessToken
	s.accountID = accountID
	s.proxyURL = proxyURL
	s.mu.Unlock()
	return s.body, 200, nil
}

func testExcelAccount() *Account {
	return &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
}

func testExcelIDToken() string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, _ := json.Marshal(map[string]any{
		"exp":                         time.Now().Add(time.Hour).Unix(),
		"email":                       "excel@example.com",
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acct-excel-42"},
	})
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestOpenAIExcelOAuthBindPersistsEncryptedCredentialAndSafeStatus(t *testing.T) {
	account := testExcelAccount()
	repo := &excelOAuthCredentialRepoStub{}
	oauth := NewOpenAIOAuthService(nil, &excelOAuthClientStub{})
	defer oauth.Stop()
	var privacyClientCalls atomic.Int32
	oauth.SetPrivacyClientFactory(func(string) (*req.Client, error) {
		privacyClientCalls.Add(1)
		return nil, errors.New("Excel token must not call Codex APIs")
	})
	service := NewOpenAIExcelOAuthService(repo, &excelOAuthAccountReaderStub{account: account}, oauth, excelOAuthEncryptorStub{}, &excelOAuthAccessStub{})

	authURL, err := service.GenerateAuthURL(context.Background(), account.ID)
	require.NoError(t, err)
	session, ok := oauth.sessionStore.Get(authURL.SessionID)
	require.True(t, ok)
	status, err := service.BindAuthorization(context.Background(), account.ID, &OpenAIExchangeCodeInput{SessionID: authURL.SessionID, Code: "code", State: session.State}, 0)
	require.NoError(t, err)
	require.Zero(t, privacyClientCalls.Load())
	require.True(t, status.Authorized)
	require.EqualValues(t, 1, status.Revision)
	require.NotContains(t, repo.record.CredentialsEncrypted, "exchange-access")
	require.NotContains(t, repo.record.CredentialsEncrypted, "exchange-refresh")

	safe, err := service.GetStatus(context.Background(), account.ID)
	require.NoError(t, err)
	encoded, err := json.Marshal(safe)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "exchange-access")
	require.NotContains(t, string(encoded), "exchange-refresh")
}

func TestOpenAIExcelOAuthDiscoverCapabilitiesUsesStoredTokenAndProjectsSafeModels(t *testing.T) {
	account := testExcelAccount()
	credentialPayload, err := json.Marshal(openAIExcelOAuthCredentialPayload{AccessToken: "excel-access-only", ChatGPTAccountID: "acct-excel-42", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	require.NoError(t, err)
	encrypted, err := (excelOAuthEncryptorStub{}).Encrypt(string(credentialPayload))
	require.NoError(t, err)
	repo := &excelOAuthCredentialRepoStub{record: &OpenAIExcelOAuthCredentialRecord{AccountID: account.ID, CredentialsEncrypted: encrypted, Revision: 1}}
	access := &excelOAuthAccessStub{body: []byte(`{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","label":"GPT-5","available":true},{"id":"gpt-disabled","available":false}],"restricted_models":["gpt-secret"]},"access_token":"must-not-echo"}`)}
	oauth := NewOpenAIOAuthService(nil, &excelOAuthClientStub{})
	defer oauth.Stop()
	svc := NewOpenAIExcelOAuthService(repo, &excelOAuthAccountReaderStub{account: account}, oauth, excelOAuthEncryptorStub{}, access)

	capabilities, err := svc.DiscoverCapabilities(context.Background(), account.ID)
	require.NoError(t, err)
	require.True(t, capabilities.Allowed)
	require.Equal(t, []ExcelBPSCapability{{ID: "gpt-5", DisplayName: "GPT-5", ReasoningEfforts: []string{}}}, capabilities.Models)
	require.Equal(t, "excel-access-only", access.accessToken)
	require.Equal(t, "acct-excel-42", access.accountID)
	encoded, err := json.Marshal(capabilities)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "excel-access-only")
	require.NotContains(t, string(encoded), "must-not-echo")
	require.NotContains(t, string(encoded), "gpt-secret")
}

func TestOpenAIOAuthServiceExcelExchangeAndRefreshSkipCodexEnrichment(t *testing.T) {
	client := &excelOAuthClientStub{}
	oauth := NewOpenAIOAuthService(nil, client)
	defer oauth.Stop()
	var privacyClientCalls atomic.Int32
	oauth.SetPrivacyClientFactory(func(string) (*req.Client, error) {
		privacyClientCalls.Add(1)
		return nil, errors.New("unexpected Codex API call")
	})

	session := &openai.OAuthSession{State: "excel-state", CodeVerifier: "verifier", ClientID: openai.ExcelClientID, TargetAccountID: 42, RedirectURI: openai.ExcelRedirectURI, CreatedAt: time.Now()}
	oauth.sessionStore.Set("excel-session", session)
	_, err := oauth.ExchangeCode(context.Background(), &OpenAIExchangeCodeInput{SessionID: "excel-session", Code: "code", State: session.State, TargetAccountID: 42})
	require.NoError(t, err)
	_, err = oauth.RefreshTokenWithClientID(context.Background(), "refresh", "", openai.ExcelClientID)
	require.NoError(t, err)
	require.Zero(t, privacyClientCalls.Load())
}

func TestOpenAIExcelOAuthRejectsShadowAndPATAccounts(t *testing.T) {
	oauth := NewOpenAIOAuthService(nil, &excelOAuthClientStub{})
	defer oauth.Stop()
	for name, account := range map[string]*Account{
		"shadow": {ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: excelPtrInt64(1)},
		"pat":    {ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"auth_mode": "personal_access_token"}},
	} {
		t.Run(name, func(t *testing.T) {
			svc := NewOpenAIExcelOAuthService(&excelOAuthCredentialRepoStub{}, &excelOAuthAccountReaderStub{account: account}, oauth, excelOAuthEncryptorStub{}, &excelOAuthAccessStub{})
			_, err := svc.GenerateAuthURL(context.Background(), account.ID)
			require.Error(t, err)
		})
	}
}

func TestOpenAIExcelOAuthRefreshSingleflight(t *testing.T) {
	account := testExcelAccount()
	encryptor := excelOAuthEncryptorStub{}
	expires := time.Now().Add(time.Minute).Unix()
	encoded, err := json.Marshal(openAIExcelOAuthCredentialPayload{AccessToken: "old-access", RefreshToken: "old-refresh", ChatGPTAccountID: "acct-excel-42", ExpiresAt: expires})
	require.NoError(t, err)
	ciphertext, err := encryptor.Encrypt(string(encoded))
	require.NoError(t, err)
	repo := &excelOAuthCredentialRepoStub{record: &OpenAIExcelOAuthCredentialRecord{AccountID: account.ID, CredentialsEncrypted: ciphertext, Revision: 1}}
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	oauthClient := &excelOAuthClientStub{refresh: func() *openai.TokenResponse {
		select {
		case <-refreshStarted:
		default:
			close(refreshStarted)
		}
		<-releaseRefresh
		return &openai.TokenResponse{AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresIn: 3600}
	}}
	oauth := NewOpenAIOAuthService(nil, oauthClient)
	defer oauth.Stop()
	access := &excelOAuthAccessStub{body: []byte(`{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`)}
	svc := NewOpenAIExcelOAuthService(repo, &excelOAuthAccountReaderStub{account: account}, oauth, encryptor, access)

	results := make(chan error, 2)
	go func() { _, e := svc.DiscoverCapabilities(context.Background(), account.ID); results <- e }()
	<-refreshStarted
	go func() { _, e := svc.DiscoverCapabilities(context.Background(), account.ID); results <- e }()
	time.Sleep(10 * time.Millisecond)
	close(releaseRefresh)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	require.EqualValues(t, 1, atomic.LoadInt32(&oauthClient.refreshCalls))
}

func excelPtrInt64(value int64) *int64 { return &value }
