package handler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func finishRequestLedgerWSTurn(ctx context.Context, result *service.OpenAIForwardResult, turnErr error) {
	var failover *service.UpstreamFailoverError
	if errors.As(turnErr, &failover) {
		return
	} // The same admitted turn may retry.
	state := requestledger.Outcome(0, turnErr)
	if turnErr == nil && (result == nil || result.UpstreamTerminalEvent == "") {
		state = "interrupted"
	}
	if result != nil {
		switch result.UpstreamTerminalEvent {
		case "response.failed", "response.incomplete":
			state = "failed"
		case "response.cancelled":
			state = "cancelled"
		}
	}
	if err := requestledger.FinishTurn(ctx, state, turnErr); err != nil {
		slog.Error("request_ledger_ws_turn_finish_failed")
	}
}
