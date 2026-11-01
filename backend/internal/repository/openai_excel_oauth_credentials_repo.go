package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAIExcelOAuthCredentialRepository struct {
	db *sql.DB
}

func NewOpenAIExcelOAuthCredentialRepository(db *sql.DB) service.OpenAIExcelOAuthCredentialRepository {
	return &openAIExcelOAuthCredentialRepository{db: db}
}

func (r *openAIExcelOAuthCredentialRepository) GetExcelOAuthCredentials(ctx context.Context, accountID int64) (*service.OpenAIExcelOAuthCredentialRecord, error) {
	var record service.OpenAIExcelOAuthCredentialRecord
	var expiresAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT account_id, credentials_encrypted, expires_at, revision, updated_at
		FROM openai_excel_oauth_credentials
		WHERE account_id = $1`, accountID).Scan(
		&record.AccountID, &record.CredentialsEncrypted, &expiresAt, &record.Revision, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		record.ExpiresAt = &expiresAt.Time
	}
	return &record, nil
}

func (r *openAIExcelOAuthCredentialRepository) SaveExcelOAuthCredentials(ctx context.Context, accountID, expectedRevision int64, ciphertext string, expiresAt *time.Time) (int64, error) {
	if accountID <= 0 || expectedRevision < 0 || strings.TrimSpace(ciphertext) == "" || json.Valid([]byte(ciphertext)) {
		return 0, errors.New("invalid Excel OAuth credential write")
	}
	if expectedRevision == 0 {
		var revision int64
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO openai_excel_oauth_credentials (account_id, credentials_encrypted, expires_at)
			VALUES ($1, $2, $3)
			ON CONFLICT (account_id) DO NOTHING
			RETURNING revision`, accountID, ciphertext, expiresAt).Scan(&revision)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, service.ErrOpenAIExcelOAuthRevisionConflict
		}
		return revision, err
	}
	var revision int64
	err := r.db.QueryRowContext(ctx, `
		UPDATE openai_excel_oauth_credentials
		SET credentials_encrypted = $2, expires_at = $3, revision = revision + 1, updated_at = NOW()
		WHERE account_id = $1 AND revision = $4
		RETURNING revision`, accountID, ciphertext, expiresAt, expectedRevision).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, service.ErrOpenAIExcelOAuthRevisionConflict
	}
	return revision, err
}

// openAIExcelOAuthAccessClient performs only capability discovery. It never
// logs the bearer or provider response and returns the raw body only to the
// service-layer parser, which projects it to an allowlist.
type openAIExcelOAuthAccessClient struct{ endpoint string }

func NewOpenAIExcelOAuthAccessClient() service.OpenAIExcelOAuthAccessClient {
	return &openAIExcelOAuthAccessClient{endpoint: "https://bps.openai.com/basispoints/api/responses/access"}
}

func (c *openAIExcelOAuthAccessClient) FetchAccessModels(ctx context.Context, accessToken, accountID, proxyURL string) ([]byte, int, error) {
	accessToken = strings.TrimSpace(accessToken)
	accountID = strings.TrimSpace(accountID)
	if accessToken == "" || accountID == "" {
		return nil, 0, errors.New("excel OAuth access token and account identity are required")
	}
	endpointURL, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, 0, err
	}
	query := endpointURL.Query()
	query.Set("include_models", "true")
	endpointURL.RawQuery = query.Encode()
	client, err := getSharedReqClient(reqClientOptions{ProxyURL: proxyURL, Timeout: 15 * time.Second})
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("Chatgpt-Account-Id", accountID).
		SetHeader("X-Openai-Account-Id", accountID).
		SetHeader("X-Basispoints-Auth-Mode", "chatgpt").
		SetHeader("Accept", "application/json").
		SetHeader("Origin", "https://bps.openai.com").
		SetHeader("X-Openai-Internal-Basispoints-Client-Product", "basispoints-excel-plugin").
		SetHeader("X-Openai-Internal-Basispoints-Client-Agent-Profile", "excel").
		SetHeader("User-Agent", "Mozilla/5.0").
		DisableAutoReadResponse().
		Get(endpointURL.String())
	if err != nil {
		return nil, 0, err
	}
	if resp == nil || resp.Response == nil || resp.Body == nil {
		return nil, 0, errors.New("excel capability response body is unavailable")
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20+1))
	if readErr != nil {
		return nil, resp.StatusCode, readErr
	}
	if len(body) > 2<<20 {
		return nil, resp.StatusCode, errors.New("excel capability response exceeds size limit")
	}
	return body, resp.StatusCode, nil
}
