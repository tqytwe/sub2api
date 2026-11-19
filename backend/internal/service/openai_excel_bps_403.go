package service

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type AccountExcelBPSRepository interface {
	DisableExcelBPSOn403(context.Context, *Account) (bool, error)
}

type AccountExcelBPSGroupRepository interface {
	MoveExcelBPSOn403(context.Context, *Account) (bool, error)
}

func (s *OpenAIGatewayService) handleExcelBPSAccessRejection(ctx context.Context, account *Account, status int, code, model string) string {
	const unchanged = "Excel BPS rejected this request; account scheduling was not changed"
	if ctx.Value(qualityIsolationProbeKey{}) == true || ctx.Err() != nil {
		return unchanged
	}
	if isExcelBPSModelPermissionError(status, code) {
		s.applyExcelBPSModelCooldown(ctx, account, model)
		return "This model is not available on the account's Excel BPS endpoint"
	}
	if account.AuthConnectionID != "" {
		if s.excelOAuthReauth != nil && (status == http.StatusUnauthorized || status == http.StatusForbidden) {
			_ = s.excelOAuthReauth.FailAuthConnection(ctx, account, status)
		}
		return unchanged
	}
	if status != http.StatusForbidden {
		return unchanged
	}
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	var moved, disabled bool
	if _, enabled := account.ExcelBPS403GroupTarget(); enabled {
		if repo, ok := s.accountRepo.(AccountExcelBPSGroupRepository); ok {
			var err error
			moved, err = repo.MoveExcelBPSOn403(stateCtx, account)
			if err != nil {
				logger.LegacyPrintf("service.openai_excel_bps", "403 group action failed: account_id=%d error_type=%T", account.ID, err)
			}
		}
	}
	if account.IsExcelBPSAutoDisableOn403Enabled() {
		if repo, ok := s.accountRepo.(AccountExcelBPSRepository); ok {
			var err error
			disabled, err = repo.DisableExcelBPSOn403(stateCtx, account)
			if err != nil {
				logger.LegacyPrintf("service.openai_excel_bps", "403 disable failed: account_id=%d error_type=%T", account.ID, err)
			}
		}
	}
	switch {
	case moved && disabled:
		return "Excel BPS was disabled and the account's groups were updated after the upstream rejection"
	case disabled:
		return "Excel BPS was disabled for this account after the upstream rejection"
	case moved:
		return "The account's groups were updated after the Excel BPS rejection"
	default:
		return unchanged
	}
}
