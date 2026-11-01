package repository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIExcelOAuthCredentialRepositoryUsesRevisionCAS(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewOpenAIExcelOAuthCredentialRepository(db).(*openAIExcelOAuthCredentialRepository)
	expires := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("INSERT INTO openai_excel_oauth_credentials").
		WithArgs(int64(42), "ciphertext-v1", &expires).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(1)))
	revision, err := repo.SaveExcelOAuthCredentials(context.Background(), 42, 0, "ciphertext-v1", &expires)
	require.NoError(t, err)
	require.EqualValues(t, 1, revision)

	mock.ExpectQuery("UPDATE openai_excel_oauth_credentials").
		WithArgs(int64(42), "ciphertext-v2", &expires, int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))
	revision, err = repo.SaveExcelOAuthCredentials(context.Background(), 42, 1, "ciphertext-v2", &expires)
	require.NoError(t, err)
	require.EqualValues(t, 2, revision)

	mock.ExpectQuery("UPDATE openai_excel_oauth_credentials").
		WithArgs(int64(42), "ciphertext-stale", &expires, int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}))
	_, err = repo.SaveExcelOAuthCredentials(context.Background(), 42, 1, "ciphertext-stale", &expires)
	require.ErrorIs(t, err, service.ErrOpenAIExcelOAuthRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIExcelOAuthCredentialRepositoryRejectsPlaintextPayload(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewOpenAIExcelOAuthCredentialRepository(db)
	_, err = repo.SaveExcelOAuthCredentials(context.Background(), 42, 0, `{"access_token":"plaintext"}`, nil)
	require.Error(t, err)
}

func TestOpenAIExcelOAuthAccessClientReadsFullBodyAndUsesProviderContract(t *testing.T) {
	const payload = `{"allowed":true,"model_catalog":{"models":[]}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/basispoints/api/responses/access", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("include_models"))
		require.Equal(t, "Bearer test-access", r.Header.Get("Authorization"))
		require.Equal(t, "acct-42", r.Header.Get("Chatgpt-Account-Id"))
		require.Equal(t, "excel", r.Header.Get("X-Openai-Internal-Basispoints-Client-Agent-Profile"))
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()
	client := &openAIExcelOAuthAccessClient{endpoint: server.URL + "/basispoints/api/responses/access"}
	body, status, err := client.FetchAccessModels(context.Background(), "test-access", "acct-42", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, payload, string(body))
}

func TestOpenAIExcelOAuthAccessClientRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", (2<<20)+1)))
	}))
	defer server.Close()
	client := &openAIExcelOAuthAccessClient{endpoint: server.URL}
	_, _, err := client.FetchAccessModels(context.Background(), "test-access", "acct-42", "")
	require.ErrorContains(t, err, "exceeds size limit")
}
