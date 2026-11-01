package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

type sessionClientRefreshStub struct {
	openaiOAuthClientRefreshStub
	clientID string
	token    string
}

func (s *sessionClientRefreshStub) RefreshTokenWithClientID(_ context.Context, token, _ string, clientID string) (*openai.TokenResponse, error) {
	s.clientID, s.token = clientID, token
	return &openai.TokenResponse{AccessToken: "new-session-access", RefreshToken: "new-session-refresh", ExpiresIn: 3600}, nil
}

func TestOpenAIOAuthSessionRefreshPreservesIssuingClient(t *testing.T) {
	client := &sessionClientRefreshStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
		"access_token": "session-access", "refresh_token": "session-refresh", "client_id": openai.ExcelClientID,
	}}
	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, openai.ExcelClientID, client.clientID)
	require.Equal(t, "session-refresh", client.token)
	credentials := svc.BuildAccountCredentials(info)
	require.Equal(t, openai.ExcelClientID, credentials["client_id"])
	require.Equal(t, "new-session-access", credentials["access_token"])
	require.Equal(t, "new-session-refresh", credentials["refresh_token"])
	require.NotEmpty(t, credentials["expires_at"])
}

func TestValidateOpenAIOAuthClientIDRejectsUnknownClient(t *testing.T) {
	got, err := ValidateOpenAIOAuthClientID("")
	require.NoError(t, err)
	require.Equal(t, openai.ClientID, got)
	got, err = ValidateOpenAIOAuthClientID(openai.ExcelClientID)
	require.NoError(t, err)
	require.Equal(t, openai.ExcelClientID, got)
	_, err = ValidateOpenAIOAuthClientID("app-untrusted")
	require.ErrorContains(t, err, "unsupported OAuth client")
}

func TestOpenAIOAuthSessionRefreshRejectsUnknownClientBeforeNetwork(t *testing.T) {
	client := &sessionClientRefreshStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
		"access_token": "session-access", "refresh_token": "session-refresh", "client_id": "app-untrusted",
	}}
	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.ErrorContains(t, err, "unsupported OAuth client")
	require.Empty(t, client.clientID)
}
