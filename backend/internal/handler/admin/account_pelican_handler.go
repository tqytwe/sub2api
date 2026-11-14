package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PelicanTestRequest struct {
	PromptMode      string                   `json:"prompt_mode"`
	PelicanQuestion *service.PelicanQuestion `json:"pelican_question"`
	ModelID         string                   `json:"model_id"`
	Prompt          string                   `json:"prompt"`
	ReasoningEffort string                   `json:"reasoning_effort"`
}

func (h *AccountHandler) PreparePelicanQuestion(c *gin.Context) {
	cfg, err := service.NewPelicanQuestion()
	if err != nil {
		response.InternalError(c, "Unable to prepare pelican question")
		return
	}
	response.Success(c, cfg)
}

func (h *AccountHandler) PelicanTest(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req PelicanTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	if len(req.Prompt) > 32000 || len(req.ModelID) > 100 {
		response.BadRequest(c, "Prompt or model is too long")
		return
	}
	if req.PromptMode == service.PelicanPromptModeRandom {
		cfg, err := service.ResolveManualPelicanQuestion(req.PelicanQuestion)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		req.Prompt = cfg.ActualPrompt
	} else if req.PromptMode != "" && req.PromptMode != "custom" {
		response.BadRequest(c, "Invalid pelican prompt mode")
		return
	} else {
		req.Prompt = strings.TrimSpace(req.Prompt) + "\n\n" + service.PelicanDeliveryContract
	}
	if err := h.accountTestService.TestPelicanAccountConnection(c, accountID, req.ModelID, req.Prompt, req.ReasoningEffort); err != nil {
		return
	}
	if h.rateLimitService != nil {
		if _, err := h.rateLimitService.RecoverAccountAfterSuccessfulTest(c.Request.Context(), accountID); err != nil {
			_ = c.Error(err)
		}
	}
}
