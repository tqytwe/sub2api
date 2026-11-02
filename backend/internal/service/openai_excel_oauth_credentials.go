package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"golang.org/x/sync/singleflight"
)

var ErrOpenAIExcelOAuthRevisionConflict = errors.New("excel OAuth credentials changed; reload status and retry")

type OpenAIExcelOAuthCredentialRecord struct {
	AccountID            int64
	CredentialsEncrypted string
	ExpiresAt            *time.Time
	Revision             int64
	UpdatedAt            time.Time
}

type OpenAIExcelOAuthCredentialRepository interface {
	GetExcelOAuthCredentials(context.Context, int64) (*OpenAIExcelOAuthCredentialRecord, error)
	SaveExcelOAuthCredentials(context.Context, int64, int64, string, *time.Time) (int64, error)
}

type OpenAIExcelOAuthAccountReader interface {
	GetAccount(context.Context, int64) (*Account, error)
	GetProxy(context.Context, int64) (*Proxy, error)
}

type OpenAIExcelOAuthAccessClient interface {
	FetchAccessModels(context.Context, string, string, string) ([]byte, int, error)
}

// OpenAIExcelOAuthStatus is safe to return to the admin UI. It contains no
// credential material, encrypted or otherwise.
type OpenAIExcelOAuthStatus struct {
	AccountID  int64      `json:"account_id"`
	Authorized bool       `json:"authorized"`
	Revision   int64      `json:"revision"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

// OpenAIExcelOAuthRuntimeCredential is the narrow credential projection used
// by the BPS text transport. It intentionally cannot be serialized to an
// account or returned by an HTTP handler.
type OpenAIExcelOAuthRuntimeCredential struct {
	AccessToken      string
	ChatGPTAccountID string
	Revision         int64
}

type openAIExcelOAuthCredentialPayload struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	IDToken          string `json:"id_token,omitempty"`
	ChatGPTAccountID string `json:"chatgpt_account_id,omitempty"`
	Email            string `json:"email,omitempty"`
	ExpiresAt        int64  `json:"expires_at,omitempty"`
}

type OpenAIExcelOAuthService struct {
	repo      OpenAIExcelOAuthCredentialRepository
	accounts  OpenAIExcelOAuthAccountReader
	oauth     *OpenAIOAuthService
	encryptor SecretEncryptor
	access    OpenAIExcelOAuthAccessClient
	clock     func() time.Time
	refresh   singleflight.Group
	capMu     sync.Mutex
	capCache  map[int64]excelOAuthCapabilityCacheEntry
}

type excelOAuthCapabilityCacheEntry struct {
	capabilities ExcelBPSCapabilities
	revision     int64
	loadedAt     time.Time
}

const excelOAuthCapabilityCacheTTL = time.Minute

func NewOpenAIExcelOAuthService(
	repo OpenAIExcelOAuthCredentialRepository,
	accounts OpenAIExcelOAuthAccountReader,
	oauth *OpenAIOAuthService,
	encryptor SecretEncryptor,
	access OpenAIExcelOAuthAccessClient,
) *OpenAIExcelOAuthService {
	return &OpenAIExcelOAuthService{repo: repo, accounts: accounts, oauth: oauth, encryptor: encryptor, access: access, clock: time.Now, capCache: make(map[int64]excelOAuthCapabilityCacheEntry)}
}

func (s *OpenAIExcelOAuthService) GenerateAuthURL(ctx context.Context, accountID int64) (*OpenAIAuthURLResult, error) {
	account, err := s.openAIOAuthAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.oauth.GenerateExcelAuthURL(ctx, account.ProxyID, account.ID)
}

func (s *OpenAIExcelOAuthService) BindAuthorization(ctx context.Context, accountID int64, input *OpenAIExchangeCodeInput, expectedRevision int64) (*OpenAIExcelOAuthStatus, error) {
	if input == nil {
		return nil, infraerrors.New(http.StatusBadRequest, "OPENAI_EXCEL_OAUTH_INVALID_REQUEST", "authorization callback is required")
	}
	if _, err := s.openAIOAuthAccount(ctx, accountID); err != nil {
		return nil, err
	}
	if expectedRevision < 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "OPENAI_EXCEL_OAUTH_INVALID_REVISION", "revision must be zero or greater")
	}
	input.TargetAccountID = accountID
	tokenInfo, err := s.oauth.ExchangeCode(ctx, input)
	if err != nil {
		return nil, err
	}
	payload := openAIExcelOAuthCredentialPayload{
		AccessToken:      tokenInfo.AccessToken,
		RefreshToken:     tokenInfo.RefreshToken,
		IDToken:          tokenInfo.IDToken,
		ChatGPTAccountID: strings.TrimSpace(tokenInfo.ChatGPTAccountID),
		Email:            strings.TrimSpace(tokenInfo.Email),
		ExpiresAt:        tokenInfo.ExpiresAt,
	}
	if payload.AccessToken == "" || payload.ChatGPTAccountID == "" {
		return nil, infraerrors.New(http.StatusBadGateway, "OPENAI_EXCEL_OAUTH_ACCOUNT_ID_MISSING", "Excel OAuth did not provide an account identity")
	}
	status, err := s.encryptAndSave(ctx, accountID, expectedRevision, payload)
	if err == nil {
		s.capMu.Lock()
		delete(s.capCache, accountID)
		s.capMu.Unlock()
	}
	return status, err
}

func (s *OpenAIExcelOAuthService) GetStatus(ctx context.Context, accountID int64) (*OpenAIExcelOAuthStatus, error) {
	if _, err := s.openAIOAuthAccount(ctx, accountID); err != nil {
		return nil, err
	}
	record, err := s.repo.GetExcelOAuthCredentials(ctx, accountID)
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_EXCEL_OAUTH_STATUS_FAILED", "could not load Excel OAuth status")
	}
	if record == nil {
		return &OpenAIExcelOAuthStatus{AccountID: accountID}, nil
	}
	return &OpenAIExcelOAuthStatus{AccountID: accountID, Authorized: true, Revision: record.Revision, ExpiresAt: record.ExpiresAt, UpdatedAt: &record.UpdatedAt}, nil
}

func (s *OpenAIExcelOAuthService) DiscoverCapabilities(ctx context.Context, accountID int64) (*ExcelBPSCapabilities, error) {
	credential, err := s.RuntimeCredential(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.discoverCapabilitiesWithCredential(ctx, accountID, credential)
}

func (s *OpenAIExcelOAuthService) discoverCapabilitiesWithCredential(ctx context.Context, accountID int64, credential *OpenAIExcelOAuthRuntimeCredential) (*ExcelBPSCapabilities, error) {
	account, err := s.openAIOAuthAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if credential == nil || credential.Revision <= 0 || strings.TrimSpace(credential.AccessToken) == "" || strings.TrimSpace(credential.ChatGPTAccountID) == "" {
		return nil, infraerrors.New(http.StatusUnauthorized, "OPENAI_EXCEL_OAUTH_ACCOUNT_ID_MISSING", "Excel OAuth authorization is incomplete; reconnect this account")
	}
	proxyURL, err := s.accountProxyURL(ctx, account)
	if err != nil {
		return nil, err
	}
	body, status, err := s.access.FetchAccessModels(ctx, credential.AccessToken, credential.ChatGPTAccountID, proxyURL)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "OPENAI_EXCEL_BPS_DISCOVERY_FAILED", "Excel model capability discovery failed")
	}
	if status != http.StatusOK {
		code := "OPENAI_EXCEL_BPS_DISCOVERY_FAILED"
		httpStatus := http.StatusBadGateway
		switch status {
		case http.StatusUnauthorized, http.StatusForbidden:
			code, httpStatus = "OPENAI_EXCEL_OAUTH_REAUTH_REQUIRED", http.StatusUnauthorized
		case http.StatusTooManyRequests:
			code, httpStatus = "OPENAI_EXCEL_BPS_RATE_LIMITED", http.StatusBadGateway
		}
		return nil, infraerrors.New(httpStatus, code, "Excel model capability discovery was rejected by the provider")
	}
	capabilities, err := ParseExcelBPSCapabilities(body)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "OPENAI_EXCEL_BPS_DISCOVERY_INVALID", "provider returned an invalid capability catalog")
	}
	s.capMu.Lock()
	s.capCache[accountID] = excelOAuthCapabilityCacheEntry{capabilities: capabilities, revision: credential.Revision, loadedAt: s.clock()}
	s.capMu.Unlock()
	return &capabilities, nil
}

// EnsureModelAllowed applies the explicit upstream model allowlist before a
// request is submitted. The short cache prevents a discovery call per turn
// while still reflecting provider access changes promptly.
func (s *OpenAIExcelOAuthService) EnsureModelAllowed(ctx context.Context, accountID int64, model string, credential *OpenAIExcelOAuthRuntimeCredential) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return infraerrors.New(http.StatusBadRequest, "OPENAI_EXCEL_BPS_MODEL_REQUIRED", "Excel BPS requires a model")
	}
	if credential == nil || credential.Revision <= 0 {
		return infraerrors.New(http.StatusUnauthorized, "OPENAI_EXCEL_OAUTH_NOT_BOUND", "Excel OAuth is not connected to this account")
	}
	now := s.clock()
	s.capMu.Lock()
	entry, ok := s.capCache[accountID]
	s.capMu.Unlock()
	if !ok || entry.revision != credential.Revision || now.Sub(entry.loadedAt) >= excelOAuthCapabilityCacheTTL {
		capabilities, err := s.discoverCapabilitiesWithCredential(ctx, accountID, credential)
		if err != nil {
			return err
		}
		entry = excelOAuthCapabilityCacheEntry{capabilities: *capabilities, revision: credential.Revision, loadedAt: now}
	}
	if !entry.capabilities.AllowsModel(model) {
		return infraerrors.New(http.StatusForbidden, "OPENAI_EXCEL_BPS_MODEL_NOT_ALLOWED", "the requested model is not allowed for this Excel BPS account")
	}
	return nil
}

// RuntimeCredential loads the separately authorized Excel grant, refreshing it
// through the existing CAS/singleflight path when it is near expiry.
func (s *OpenAIExcelOAuthService) RuntimeCredential(ctx context.Context, accountID int64) (*OpenAIExcelOAuthRuntimeCredential, error) {
	account, err := s.openAIOAuthAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.GetExcelOAuthCredentials(ctx, accountID)
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_EXCEL_OAUTH_READ_FAILED", "could not read Excel OAuth credentials")
	}
	if record == nil || strings.TrimSpace(record.CredentialsEncrypted) == "" {
		return nil, infraerrors.New(http.StatusConflict, "OPENAI_EXCEL_OAUTH_NOT_BOUND", "Excel OAuth is not connected to this account")
	}
	payload, err := s.decryptPayload(record.CredentialsEncrypted)
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_EXCEL_OAUTH_DECRYPT_FAILED", "Excel OAuth credentials could not be decrypted")
	}
	if payload.ExpiresAt > 0 && payload.ExpiresAt <= s.clock().Add(2*time.Minute).Unix() {
		payload, record, err = s.refreshPayloadSingleflight(ctx, account, record, payload)
		if err != nil {
			return nil, infraerrors.New(http.StatusUnauthorized, "OPENAI_EXCEL_OAUTH_REAUTH_REQUIRED", "Excel OAuth authorization has expired; reconnect this account")
		}
	}
	if strings.TrimSpace(payload.AccessToken) == "" || strings.TrimSpace(payload.ChatGPTAccountID) == "" {
		return nil, infraerrors.New(http.StatusUnauthorized, "OPENAI_EXCEL_OAUTH_ACCOUNT_ID_MISSING", "Excel OAuth authorization is incomplete; reconnect this account")
	}
	return &OpenAIExcelOAuthRuntimeCredential{
		AccessToken:      strings.TrimSpace(payload.AccessToken),
		ChatGPTAccountID: strings.TrimSpace(payload.ChatGPTAccountID),
		Revision:         record.Revision,
	}, nil
}

// refreshPayloadSingleflight serializes refresh-token consumption per account
// within a process. It re-reads the credential row inside the flight so a
// caller that arrived after another refresh reuses the newer CAS revision
// instead of spending the same refresh token again.
func (s *OpenAIExcelOAuthService) refreshPayloadSingleflight(ctx context.Context, account *Account, record *OpenAIExcelOAuthCredentialRecord, payload openAIExcelOAuthCredentialPayload) (openAIExcelOAuthCredentialPayload, *OpenAIExcelOAuthCredentialRecord, error) {
	key := strconv.FormatInt(account.ID, 10)
	value, err, _ := s.refresh.Do(key, func() (any, error) {
		latest, err := s.repo.GetExcelOAuthCredentials(ctx, account.ID)
		if err != nil {
			return nil, err
		}
		if latest == nil || strings.TrimSpace(latest.CredentialsEncrypted) == "" {
			return nil, errors.New("excel OAuth credentials are missing")
		}
		latestPayload := payload
		if latest.Revision != record.Revision || latest.CredentialsEncrypted != record.CredentialsEncrypted {
			latestPayload, err = s.decryptPayload(latest.CredentialsEncrypted)
			if err != nil {
				return nil, err
			}
		}
		if latestPayload.ExpiresAt == 0 || latestPayload.ExpiresAt > s.clock().Add(2*time.Minute).Unix() {
			return excelOAuthRefreshResult{payload: latestPayload, record: latest}, nil
		}
		refreshedPayload, refreshedRecord, err := s.refreshPayload(ctx, account, latest, latestPayload)
		if err != nil {
			return nil, err
		}
		return excelOAuthRefreshResult{payload: refreshedPayload, record: refreshedRecord}, nil
	})
	if err != nil {
		return payload, record, err
	}
	result, ok := value.(excelOAuthRefreshResult)
	if !ok {
		return payload, record, errors.New("invalid Excel OAuth refresh result")
	}
	return result.payload, result.record, nil
}

type excelOAuthRefreshResult struct {
	payload openAIExcelOAuthCredentialPayload
	record  *OpenAIExcelOAuthCredentialRecord
}

func (s *OpenAIExcelOAuthService) openAIOAuthAccount(ctx context.Context, accountID int64) (*Account, error) {
	if accountID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "OPENAI_EXCEL_OAUTH_INVALID_ACCOUNT", "account_id must be positive")
	}
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth || account.IsCredentialShadow() || account.IsOpenAIPersonalAccessToken() {
		return nil, infraerrors.New(http.StatusBadRequest, "OPENAI_EXCEL_OAUTH_INVALID_ACCOUNT", "Excel OAuth requires an existing OpenAI OAuth account")
	}
	return account, nil
}

func (s *OpenAIExcelOAuthService) encryptAndSave(ctx context.Context, accountID, expectedRevision int64, payload openAIExcelOAuthCredentialPayload) (*OpenAIExcelOAuthStatus, error) {
	if s.encryptor == nil {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "OPENAI_EXCEL_OAUTH_ENCRYPTION_UNAVAILABLE", "credential encryption is unavailable")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_EXCEL_OAUTH_ENCRYPT_FAILED", "could not encode Excel OAuth credentials")
	}
	ciphertext, err := s.encryptor.Encrypt(string(encoded))
	if err != nil || ciphertext == "" || ciphertext == string(encoded) {
		return nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_EXCEL_OAUTH_ENCRYPT_FAILED", "could not encrypt Excel OAuth credentials")
	}
	var expiresAt *time.Time
	if payload.ExpiresAt > 0 {
		expires := time.Unix(payload.ExpiresAt, 0).UTC()
		expiresAt = &expires
	}
	revision, err := s.repo.SaveExcelOAuthCredentials(ctx, accountID, expectedRevision, ciphertext, expiresAt)
	if errors.Is(err, ErrOpenAIExcelOAuthRevisionConflict) {
		return nil, infraerrors.Conflict("OPENAI_EXCEL_OAUTH_REVISION_CONFLICT", "Excel OAuth credentials changed; reload status and retry")
	}
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_EXCEL_OAUTH_SAVE_FAILED", "could not save Excel OAuth credentials")
	}
	return &OpenAIExcelOAuthStatus{AccountID: accountID, Authorized: true, Revision: revision, ExpiresAt: expiresAt}, nil
}

func (s *OpenAIExcelOAuthService) decryptPayload(ciphertext string) (openAIExcelOAuthCredentialPayload, error) {
	var payload openAIExcelOAuthCredentialPayload
	if s.encryptor == nil {
		return payload, errors.New("credential encryption is unavailable")
	}
	plain, err := s.encryptor.Decrypt(ciphertext)
	if err != nil {
		return payload, err
	}
	err = json.Unmarshal([]byte(plain), &payload)
	return payload, err
}

func (s *OpenAIExcelOAuthService) refreshPayload(ctx context.Context, account *Account, record *OpenAIExcelOAuthCredentialRecord, payload openAIExcelOAuthCredentialPayload) (openAIExcelOAuthCredentialPayload, *OpenAIExcelOAuthCredentialRecord, error) {
	if strings.TrimSpace(payload.RefreshToken) == "" {
		return payload, record, errors.New("excel OAuth refresh token is missing")
	}
	proxyURL, err := s.accountProxyURL(ctx, account)
	if err != nil {
		return payload, record, err
	}
	refreshed, err := s.oauth.RefreshTokenWithClientID(ctx, payload.RefreshToken, proxyURL, openai.ExcelClientID)
	if err != nil || refreshed == nil || strings.TrimSpace(refreshed.AccessToken) == "" {
		return payload, record, errors.New("excel OAuth refresh failed")
	}
	payload.AccessToken = refreshed.AccessToken
	if strings.TrimSpace(refreshed.RefreshToken) != "" {
		payload.RefreshToken = refreshed.RefreshToken
	}
	if strings.TrimSpace(refreshed.IDToken) != "" {
		payload.IDToken = refreshed.IDToken
	}
	if strings.TrimSpace(refreshed.ChatGPTAccountID) != "" {
		payload.ChatGPTAccountID = strings.TrimSpace(refreshed.ChatGPTAccountID)
	}
	if refreshed.ExpiresAt > 0 {
		payload.ExpiresAt = refreshed.ExpiresAt
	} else if refreshed.ExpiresIn > 0 {
		payload.ExpiresAt = s.clock().Add(time.Duration(refreshed.ExpiresIn) * time.Second).Unix()
	}
	status, err := s.encryptAndSave(ctx, account.ID, record.Revision, payload)
	if err != nil {
		return payload, record, err
	}
	record.Revision = status.Revision
	record.ExpiresAt = status.ExpiresAt
	return payload, record, nil
}

func (s *OpenAIExcelOAuthService) accountProxyURL(ctx context.Context, account *Account) (string, error) {
	if account.ProxyID == nil {
		return "", nil
	}
	proxy, err := s.accounts.GetProxy(ctx, *account.ProxyID)
	if err != nil || proxy == nil || !proxy.IsActive() || proxy.IsExpired(s.clock()) {
		return "", infraerrors.New(http.StatusBadRequest, "OPENAI_EXCEL_OAUTH_PROXY_UNAVAILABLE", "account proxy is unavailable")
	}
	return proxy.URL(), nil
}
