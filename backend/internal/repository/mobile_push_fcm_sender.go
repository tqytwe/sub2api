package repository

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	fcmMessagingScope = "https://www.googleapis.com/auth/firebase.messaging"
	fcmDefaultBaseURL = "https://fcm.googleapis.com"
)

type fcmServiceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type FCMHTTPSender struct {
	configured bool
	projectID  string
	baseURL    string
	account    fcmServiceAccount
	privateKey *rsa.PrivateKey
	httpClient *http.Client
	now        func() time.Time

	tokenMu        sync.Mutex
	accessToken    string
	accessTokenExp time.Time
}

func NewFCMHTTPSender(cfg config.MobilePushConfig, httpClient *http.Client) (*FCMHTTPSender, error) {
	cfg = cfg.Normalized()
	sender := &FCMHTTPSender{httpClient: httpClient, now: time.Now, baseURL: cfg.APIBaseURL}
	if sender.httpClient == nil {
		sender.httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if sender.baseURL == "" {
		sender.baseURL = fcmDefaultBaseURL
	}
	if !cfg.Enabled || (cfg.ServiceAccountFile == "" && cfg.ServiceAccountJSON == "") {
		return sender, nil
	}
	raw := []byte(cfg.ServiceAccountJSON)
	if len(raw) == 0 {
		var err error
		raw, err = os.ReadFile(cfg.ServiceAccountFile)
		if err != nil {
			return nil, fmt.Errorf("read FCM service account file: %w", err)
		}
	}
	if err := json.Unmarshal(raw, &sender.account); err != nil {
		return nil, errors.New("parse FCM service account JSON")
	}
	sender.projectID = cfg.ProjectID
	if sender.projectID == "" {
		sender.projectID = strings.TrimSpace(sender.account.ProjectID)
	}
	if sender.projectID == "" || strings.TrimSpace(sender.account.ClientEmail) == "" || strings.TrimSpace(sender.account.PrivateKey) == "" || strings.TrimSpace(sender.account.TokenURI) == "" {
		return nil, errors.New("FCM service account is missing required fields")
	}
	privateKey, err := parseFCMPrivateKey(sender.account.PrivateKey)
	if err != nil {
		return nil, errors.New("parse FCM service account private key")
	}
	sender.privateKey = privateKey
	sender.configured = true
	return sender, nil
}

func (s *FCMHTTPSender) Configured() bool {
	return s != nil && s.configured
}

func (s *FCMHTTPSender) Send(ctx context.Context, token string, delivery service.MobilePushDelivery) error {
	if !s.Configured() {
		return service.ErrFCMNotConfigured
	}
	accessToken, err := s.oauthAccessToken(ctx)
	if err != nil {
		return err
	}
	data := make(map[string]string, len(delivery.Data)+3)
	for key, value := range delivery.Data {
		data[key] = value
	}
	if delivery.EventType != "" {
		data["event_type"] = delivery.EventType
	}
	if delivery.SourceType != "" {
		data["source_type"] = delivery.SourceType
	}
	if delivery.SourceID != "" {
		data["source_id"] = delivery.SourceID
	}
	tag := strings.Trim(strings.TrimSpace(delivery.SourceType)+":"+strings.TrimSpace(delivery.SourceID), ":")
	android := map[string]any{"priority": "high"}
	if tag != "" {
		android["collapse_key"] = tag
		android["notification"] = map[string]string{"tag": tag}
	}
	payload, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"token":        token,
			"notification": map[string]string{"title": delivery.TitleZh, "body": delivery.BodyZh},
			"data":         data,
			"android":      android,
		},
	})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/v1/projects/%s/messages:send", strings.TrimRight(s.baseURL, "/"), url.PathEscape(s.projectID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("FCM HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if fcmResponseHasPermanentTokenError(responseBody) {
			return service.ErrFCMTokenInvalid
		}
		return fmt.Errorf("FCM HTTP request returned status %d", resp.StatusCode)
	}
	return nil
}

func fcmResponseHasPermanentTokenError(body []byte) bool {
	var response struct {
		Error struct {
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &response) != nil {
		return false
	}
	for _, detail := range response.Error.Details {
		if strings.EqualFold(strings.TrimSpace(detail.ErrorCode), "UNREGISTERED") {
			return true
		}
	}
	return false
}

func (s *FCMHTTPSender) oauthAccessToken(ctx context.Context) (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	now := s.now().UTC()
	if s.accessToken != "" && now.Add(time.Minute).Before(s.accessTokenExp) {
		return s.accessToken, nil
	}
	assertion, err := s.signedJWT(now)
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.account.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("FCM OAuth request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
		return "", fmt.Errorf("FCM OAuth request returned status %d", resp.StatusCode)
	}
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 32<<10))
	if err := decoder.Decode(&tokenResponse); err != nil {
		return "", errors.New("decode FCM OAuth response")
	}
	if tokenResponse.AccessToken == "" {
		return "", errors.New("FCM OAuth response did not include access token")
	}
	if tokenResponse.ExpiresIn <= 0 {
		tokenResponse.ExpiresIn = 3600
	}
	s.accessToken = tokenResponse.AccessToken
	s.accessTokenExp = now.Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	return s.accessToken, nil
}

func (s *FCMHTTPSender) signedJWT(now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss": s.account.ClientEmail, "scope": fcmMessagingScope, "aud": s.account.TokenURI,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parseFCMPrivateKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, errors.New("invalid PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		return rsaKey, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

var _ service.FCMSender = (*FCMHTTPSender)(nil)
