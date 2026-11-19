package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const excelBPS403RecoveryScanInterval = time.Minute
const excelBPS403RecoveryTimeout = 45 * time.Second
const excelBPS403RecoveryConcurrency = 3

// Claims persist before network I/O, preventing duplicate attempts across
// instances and preserving the configured interval after a restart.
type AccountExcelBPSRecoveryRepository interface {
	ClaimExcelBPS403Probe(context.Context, *Account, time.Time) (bool, error)
	RestoreExcelBPSAfter403(context.Context, *Account) (bool, error)
}

func (a *Account) IsExcelBPS403RecoveryPending() bool {
	if a == nil || a.Platform != PlatformOpenAI || a.Type != AccountTypeOAuth || a.IsCredentialShadow() ||
		a.IsOpenAIPersonalAccessToken() || a.IsOpenAIAgentIdentity() || a.AuthConnectionID != "" ||
		a.Extra["openai_connections_managed"] == true || strings.EqualFold(a.GetCredential("plan_type"), "free") || !a.IsActive() || !a.Schedulable || a.IsExcelBPSEnabled() {
		return false
	}
	if a.Extra[ExcelBPSAutoRecoverOn403Key] != true || a.Extra["openai_excel_bps_auto_disable_on_403"] != true {
		return false
	}
	at, _ := a.Extra[ExcelBPS403DisabledAtKey].(string)
	_, err := time.Parse(time.RFC3339Nano, at)
	return err == nil
}

func (a *Account) ExcelBPS403RecoveryDue(now time.Time) bool {
	if !a.IsExcelBPS403RecoveryPending() || (a.AutoPauseOnExpired && a.ExpiresAt != nil && !now.Before(*a.ExpiresAt)) {
		return false
	}
	disabledAtText, ok := a.Extra[ExcelBPS403DisabledAtKey].(string)
	if !ok {
		return false
	}
	disabledAt, err := time.Parse(time.RFC3339Nano, disabledAtText)
	if err != nil {
		return false
	}
	last := disabledAt
	if raw, exists := a.Extra[ExcelBPS403LastProbeAtKey]; exists {
		text, ok := raw.(string)
		at, err := time.Parse(time.RFC3339Nano, text)
		if !ok || err != nil {
			return false
		}
		if at.After(last) {
			last = at
		}
	}
	return !now.Before(last.Add(a.ExcelBPS403RecoveryInterval()))
}

func cloneExcelBPSRecoveryExtra(extra map[string]any) map[string]any {
	cloned := make(map[string]any, len(extra)+1)
	for key, value := range extra {
		cloned[key] = value
	}
	return cloned
}

func excelBPS403RecoveryModel(a *Account) string {
	raw, scoped := a.Extra["openai_excel_bps_models"]
	if !scoped {
		return openai.DefaultTestModel
	}
	var models []string
	switch values := raw.(type) {
	case []string:
		models = values
	case []any:
		for _, value := range values {
			if model, ok := value.(string); ok {
				models = append(models, model)
			}
		}
	}
	for _, model := range models {
		if model = strings.TrimSpace(model); model != "" {
			return model
		}
	}
	return ""
}

func (s *OpenAIGatewayService) StartBPS403Recovery() {
	if s == nil || s.accountRepo == nil {
		return
	}
	if _, ok := s.accountRepo.(AccountExcelBPSRecoveryRepository); !ok {
		return
	}
	s.excelBPSRecoveryMu.Lock()
	defer s.excelBPSRecoveryMu.Unlock()
	if s.excelBPSRecoveryStopped || s.excelBPSRecoveryCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.excelBPSRecoveryCancel, s.excelBPSRecoveryDone = cancel, done
	go func() {
		defer close(done)
		ticker := time.NewTicker(excelBPS403RecoveryScanInterval)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			s.syncExcelBPS403Recovery(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *OpenAIGatewayService) StopBPS403Recovery() {
	if s == nil {
		return
	}
	s.excelBPSRecoveryMu.Lock()
	s.excelBPSRecoveryStopped = true
	cancel, done := s.excelBPSRecoveryCancel, s.excelBPSRecoveryDone
	s.excelBPSRecoveryMu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (s *OpenAIGatewayService) syncExcelBPS403Recovery(ctx context.Context) {
	defer func() {
		if recover() != nil {
			logger.FromContext(ctx).Error("excel_bps.recovery_scan_panicked")
		}
	}()
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	accounts, err := s.accountRepo.ListByPlatform(queryCtx, PlatformOpenAI)
	cancel()
	if err != nil {
		if ctx.Err() == nil {
			logger.FromContext(ctx).Warn("excel_bps.recovery_scan_failed")
		}
		return
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	slots := make(chan struct{}, excelBPS403RecoveryConcurrency)
	for i := range accounts {
		account := &accounts[i]
		if !account.ExcelBPS403RecoveryDue(time.Now()) {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case slots <- struct{}{}:
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			s.recoverExcelBPS403Account(ctx, account, time.Now())
		}()
	}
}

func (s *OpenAIGatewayService) recoverExcelBPS403Account(ctx context.Context, account *Account, now time.Time) {
	defer func() {
		if recover() != nil {
			logger.FromContext(ctx).Error("excel_bps.recovery_probe_panicked", zap.Int64("account_id", account.ID))
		}
	}()
	repo, ok := s.accountRepo.(AccountExcelBPSRecoveryRepository)
	if !ok || !account.ExcelBPS403RecoveryDue(now) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, excelBPS403RecoveryTimeout)
	defer cancel()
	claimed, err := repo.ClaimExcelBPS403Probe(ctx, account, now)
	if err != nil {
		logger.FromContext(ctx).Warn("excel_bps.recovery_claim_failed", zap.Int64("account_id", account.ID))
		return
	}
	if !claimed {
		return
	}
	snapshot := *account
	snapshot.Extra = cloneExcelBPSRecoveryExtra(account.Extra)
	snapshot.Extra[ExcelBPS403LastProbeAtKey] = now.UTC().Format(time.RFC3339Nano)
	if err := s.probeExcelBPS403Recovery(ctx, &snapshot); err != nil {
		// Do not log token, proxy URL or upstream response text.
		if ctx.Err() != context.Canceled {
			logger.FromContext(ctx).Info("excel_bps.recovery_probe_failed", zap.Int64("account_id", account.ID))
		}
		return
	}
	changed, err := repo.RestoreExcelBPSAfter403(ctx, &snapshot)
	if err != nil {
		logger.FromContext(ctx).Warn("excel_bps.recovery_restore_failed", zap.Int64("account_id", account.ID))
	} else if changed {
		logger.FromContext(ctx).Info("excel_bps.recovery_restored", zap.Int64("account_id", account.ID))
	}
}

// Probe the actual authenticated BPS Responses endpoint without enabling
// production routing, replaying user requests or applying 403 group actions.
func (s *OpenAIGatewayService) probeExcelBPS403Recovery(ctx context.Context, account *Account) error {
	model := excelBPS403RecoveryModel(account)
	if model == "" || s.httpUpstream == nil {
		return errors.New("BPS recovery probe unavailable")
	}
	credential, err := s.excelBPSRuntimeCredential(ctx, account)
	if err != nil {
		return err
	}
	token, accountID := credential.AccessToken, credential.ChatGPTAccountID
	if accountID == "" {
		return errors.New("BPS recovery account identity unavailable")
	}
	nonce := uuid.NewString()
	raw, err := json.Marshal(map[string]any{
		"model": model, "stream": true, "store": false,
		"reasoning": map[string]any{"effort": "low"},
		"input":     []any{bpsProbeUserMessage("Reply with exactly this nonce and nothing else: " + nonce)},
	})
	if err != nil {
		return err
	}
	body, _, err := basispoints.Prepare(raw, "", nil)
	if err != nil {
		return err
	}
	proxy := ""
	if account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	ctx = WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileLongStream))
	req, err := newExcelBPSRequest(ctx, body, token, accountID)
	if err != nil {
		return err
	}
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return err
	}
	if resp == nil || resp.Body == nil || resp.StatusCode != http.StatusOK {
		return errors.New("BPS recovery request rejected")
	}
	const maxProbeBytes = 2 << 20
	wire, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBytes+1))
	if err != nil {
		return err
	}
	if len(wire) > maxProbeBytes {
		return errors.New("BPS recovery response too large")
	}
	response, err := parseBPSAccountProbeResponse(wire)
	if err != nil {
		return err
	}
	if !bpsProbeExactText(response, nonce) {
		return errors.New("BPS recovery response mismatch")
	}
	return nil
}

type bpsAccountProbeResponse struct {
	Status string            `json:"status"`
	Output []json.RawMessage `json:"output"`
}

type bpsAccountProbeItem struct {
	Type                  string    `json:"type"`
	Role                  string    `json:"role"`
	Name                  string    `json:"name"`
	Namespace             string    `json:"namespace"`
	CallID                string    `json:"call_id"`
	Arguments             string    `json:"arguments"`
	EncryptedFunctionArgs *[]string `json:"encrypted_function_args"`
	Content               []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func bpsProbeUserMessage(text string) map[string]any {
	return map[string]any{"type": "message", "role": "user", "content": []any{
		map[string]any{"type": "input_text", "text": text},
	}}
}

func parseBPSAccountProbeResponse(wire []byte) (bpsAccountProbeResponse, error) {
	scanner := bufio.NewScanner(bytes.NewReader(wire))
	scanner.Buffer(make([]byte, 0, 4096), 1<<20)
	var completed *bpsAccountProbeResponse
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(line[len("data:"):])
		if bytes.Equal(data, []byte("[DONE]")) {
			continue
		}
		var event struct {
			Type     string          `json:"type"`
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return bpsAccountProbeResponse{}, err
		}
		if event.Type == "response.failed" || event.Type == "response.incomplete" {
			return bpsAccountProbeResponse{}, errors.New("bps response failed")
		}
		if event.Type != "response.completed" {
			continue
		}
		if completed != nil {
			return bpsAccountProbeResponse{}, errors.New("duplicate BPS completion")
		}
		var response bpsAccountProbeResponse
		if err := json.Unmarshal(event.Response, &response); err != nil || response.Status != "completed" {
			return bpsAccountProbeResponse{}, errors.New("invalid BPS completion")
		}
		completed = &response
	}
	if err := scanner.Err(); err != nil {
		return bpsAccountProbeResponse{}, err
	}
	if completed == nil {
		return bpsAccountProbeResponse{}, errors.New("missing BPS completion")
	}
	return *completed, nil
}

func bpsProbeExactText(response bpsAccountProbeResponse, nonce string) bool {
	var text strings.Builder
	sawText := false
	for _, raw := range response.Output {
		var item bpsAccountProbeItem
		if json.Unmarshal(raw, &item) != nil {
			return false
		}
		if item.Type == "reasoning" {
			continue
		}
		if item.Type != "message" || item.Role != "assistant" {
			return false
		}
		for _, content := range item.Content {
			if content.Type != "output_text" {
				return false
			}
			sawText = true
			_, _ = text.WriteString(content.Text)
		}
	}
	return sawText && strings.TrimSpace(text.String()) == nonce
}
