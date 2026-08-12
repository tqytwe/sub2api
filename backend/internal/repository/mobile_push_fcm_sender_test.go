package repository

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFCMHTTPSenderWithoutCredentialsIsNonFatal(t *testing.T) {
	sender, err := NewFCMHTTPSender(config.MobilePushConfig{Enabled: true}, nil)
	require.NoError(t, err)
	require.False(t, sender.Configured())
	require.ErrorIs(t, sender.Send(context.Background(), "private-token", service.MobilePushDelivery{}), service.ErrFCMNotConfigured)
}

func TestProvideMobilePushSenderRequiresPersistentKeyForAnyEnabledCredential(t *testing.T) {
	_, err := ProvideMobilePushSender(
		config.MobilePushConfig{Enabled: true, ServiceAccountJSON: `{}`},
		&config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: false}},
	)
	require.ErrorContains(t, err, "persistent totp.encryption_key")
}

func TestProvideMobilePushSenderRejectsPartialOrAmbiguousCredentials(t *testing.T) {
	appCfg := &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}}

	_, err := ProvideMobilePushSender(config.MobilePushConfig{Enabled: true, ProjectID: "project-1"}, appCfg)
	require.ErrorContains(t, err, "FCM_SERVICE_ACCOUNT_FILE or FCM_SERVICE_ACCOUNT_JSON")

	_, err = ProvideMobilePushSender(config.MobilePushConfig{
		Enabled: true, ServiceAccountFile: "/tmp/fcm.json", ServiceAccountJSON: `{}`,
	}, appCfg)
	require.ErrorContains(t, err, "only one FCM service account source")
}

func TestFCMHTTPSenderExchangesJWTAndCachesAccessToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustMarshalPKCS8(t, privateKey)}))
	var tokenCalls atomic.Int32
	var sendCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			require.NoError(t, r.ParseForm())
			require.Equal(t, "urn:ietf:params:oauth:grant-type:jwt-bearer", r.Form.Get("grant_type"))
			require.Len(t, strings.Split(r.Form.Get("assertion"), "."), 3)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "oauth-access", "expires_in": 3600})
		case "/v1/projects/project-1/messages:send":
			sendCalls.Add(1)
			require.Equal(t, "Bearer oauth-access", r.Header.Get("Authorization"))
			var payload struct {
				Message struct {
					Token   string            `json:"token"`
					Data    map[string]string `json:"data"`
					Android struct {
						Notification map[string]string `json:"notification"`
					} `json:"android"`
				} `json:"message"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "fcm-private-token", payload.Message.Token)
			require.Equal(t, "task.completed", payload.Message.Data["event_type"])
			require.Equal(t, "mobile_task", payload.Message.Data["source_type"])
			require.Equal(t, "task-1", payload.Message.Data["source_id"])
			require.Equal(t, "com.jisudeng.chat.PUSH_OPEN", payload.Message.Android.Notification["click_action"])
			require.Equal(t, "mobile_task:task-1", payload.Message.Android.Notification["tag"])
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"message-id"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	credentials, err := json.Marshal(map[string]string{
		"project_id": "project-1", "client_email": "push@example.test",
		"private_key": privateKeyPEM, "token_uri": server.URL + "/token",
	})
	require.NoError(t, err)
	sender, err := NewFCMHTTPSender(config.MobilePushConfig{
		Enabled: true, ProjectID: "project-1", ServiceAccountJSON: string(credentials), APIBaseURL: server.URL,
	}, server.Client())
	require.NoError(t, err)

	delivery := service.MobilePushDelivery{
		EventType: "task.completed", SourceType: "mobile_task", SourceID: "task-1",
		TitleZh: "任务完成", BodyZh: "任务已经完成",
	}
	require.NoError(t, sender.Send(context.Background(), "fcm-private-token", delivery))
	require.NoError(t, sender.Send(context.Background(), "fcm-private-token", delivery))
	require.Equal(t, int32(1), tokenCalls.Load())
	require.Equal(t, int32(2), sendCalls.Load())
}

func TestFCMHTTPSenderErrorDoesNotExposeTokenOrResponseBody(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustMarshalPKCS8(t, privateKey)}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "oauth-access", "expires_in": 3600})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"private-token-in-response"}`))
	}))
	t.Cleanup(server.Close)
	credentials, err := json.Marshal(map[string]string{
		"project_id": "project-1", "client_email": "push@example.test",
		"private_key": privateKeyPEM, "token_uri": server.URL + "/token",
	})
	require.NoError(t, err)
	sender, err := NewFCMHTTPSender(config.MobilePushConfig{
		Enabled: true, ProjectID: "project-1", ServiceAccountJSON: string(credentials), APIBaseURL: server.URL,
	}, server.Client())
	require.NoError(t, err)

	err = sender.Send(context.Background(), "private-token", service.MobilePushDelivery{TitleZh: "标题", BodyZh: "正文"})

	require.Error(t, err)
	require.False(t, errors.Is(err, service.ErrFCMNotConfigured))
	require.NotContains(t, err.Error(), "private-token")
	require.NotContains(t, err.Error(), "private-token-in-response")
}

func TestFCMHTTPSenderClassifiesUnregisteredToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustMarshalPKCS8(t, privateKey)}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "oauth-access", "expires_in": 3600})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"details":[{"errorCode":"UNREGISTERED"}]}}`))
	}))
	t.Cleanup(server.Close)
	credentials, err := json.Marshal(map[string]string{
		"project_id": "project-1", "client_email": "push@example.test",
		"private_key": privateKeyPEM, "token_uri": server.URL + "/token",
	})
	require.NoError(t, err)
	sender, err := NewFCMHTTPSender(config.MobilePushConfig{
		Enabled: true, ServiceAccountJSON: string(credentials), APIBaseURL: server.URL,
	}, server.Client())
	require.NoError(t, err)

	err = sender.Send(context.Background(), "permanently-invalid-token", service.MobilePushDelivery{TitleZh: "标题", BodyZh: "正文"})

	require.ErrorIs(t, err, service.ErrFCMTokenInvalid)
}

func mustMarshalPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return encoded
}
