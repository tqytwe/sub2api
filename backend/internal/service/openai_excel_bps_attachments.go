package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
)

func newExcelBPSRequest(ctx context.Context, body []byte, token, accountID string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, basispoints.ResponsesURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	applyExcelBPSRequestHeaders(req, token, accountID)
	return req, nil
}

type excelBPSAttachmentRejection struct {
	status     int
	headers    http.Header
	observedAt time.Time
}

func (e *excelBPSAttachmentRejection) Error() string {
	return fmt.Sprintf("BPS attachment upload returned HTTP %d", e.status)
}

func (s *OpenAIGatewayService) uploadExcelBPSAttachment(ctx context.Context, account *Account, token, accountID string, attachment basispoints.InlineAttachment) (string, error) {
	reader, contentType, length, err := attachment.Multipart()
	if err != nil {
		return "", fmt.Errorf("build BPS attachment upload")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, basispoints.AttachmentsURL, io.NopCloser(reader))
	if err != nil {
		return "", fmt.Errorf("build BPS attachment upload")
	}
	applyExcelBPSRequestHeaders(req, token, accountID)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	req.ContentLength = length
	req.GetBody = func() (io.ReadCloser, error) {
		body, _, _, bodyErr := attachment.Multipart()
		if bodyErr != nil {
			return nil, bodyErr
		}
		return io.NopCloser(body), nil
	}
	if err := s.fenceAuthConnectionEgress(ctx, req, account); err != nil {
		return "", err
	}
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	observedAt := time.Now()
	if err != nil {
		return "", fmt.Errorf("BPS attachment upload failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if account.AuthConnectionID != "" && s.excelOAuthReauth != nil &&
			(resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			_ = s.excelOAuthReauth.FailAuthConnection(ctx, account, resp.StatusCode)
		}
		return "", &excelBPSAttachmentRejection{status: resp.StatusCode, headers: resp.Header.Clone(), observedAt: observedAt}
	}
	return basispoints.ReadAttachmentID(resp.Body)
}

func (s *OpenAIGatewayService) repairExcelBPSResponse(ctx context.Context, account *Account, token, accountID, proxyURL string, body []byte) (map[string]any, error) {
	req, err := newExcelBPSRequest(ctx, body, token, accountID)
	if err != nil {
		return nil, err
	}
	if err := s.fenceAuthConnectionEgress(ctx, req, account); err != nil {
		return nil, err
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	ctx = context.WithValue(ctx, openAI429ObservedAtKey{}, time.Now())
	if err != nil {
		return nil, fmt.Errorf("BPS tool correction transport failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusTooManyRequests {
			s.applyExcelBPSCooldown(ctx, account, resp.Header)
		}
		return nil, fmt.Errorf("BPS tool correction returned HTTP %d", resp.StatusCode)
	}
	s.UpdateCodexUsageSnapshotFromHeaders(ctx, account.ID, resp.Header)
	return basispoints.ReadToolRepairResponse(s.guardExcelBPSProgress(ctx, resp.Body))
}
