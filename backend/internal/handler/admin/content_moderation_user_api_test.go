package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type moderationUsersAPISettings struct{ service.SettingRepository }

func (moderationUsersAPISettings) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}

func (moderationUsersAPISettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

type moderationUsersAPIRepository struct {
	service.ContentModerationRepository
	state        service.ModerationUserState
	listedUserID int64
	lastAction   service.ModerationUserAction
}

func (r *moderationUsersAPIRepository) GetModerationUser(_ context.Context, userID int64, _ time.Time, _ bool) (*service.ModerationUserState, error) {
	if userID != r.state.UserID {
		return nil, service.ErrUserNotFound
	}
	state := r.state
	return &state, nil
}

func (r *moderationUsersAPIRepository) ListModerationUsers(_ context.Context, _, _ int, _ string, userID int64, _ time.Time, _ bool) ([]service.ModerationUserState, int64, error) {
	r.listedUserID = userID
	if userID != r.state.UserID {
		return nil, 0, nil
	}
	return []service.ModerationUserState{r.state}, 1, nil
}

func (r *moderationUsersAPIRepository) ManageModerationUser(_ context.Context, userID int64, input service.ModerationUserAction) error {
	if userID != r.state.UserID || input.Version != r.state.Version {
		return service.ErrUserNotFound
	}
	r.lastAction = input
	if input.Exempt != nil {
		r.state.AutoBanExempt = *input.Exempt
	}
	if input.KeywordExempt != nil {
		r.state.KeywordExempt = *input.KeywordExempt
	}
	if input.APIAuditExempt != nil {
		r.state.APIAuditExempt = *input.APIAuditExempt
	}
	if input.EmailExempt != nil {
		r.state.EmailExempt = *input.EmailExempt
	}
	r.state.Version++
	r.state.Reason = input.Reason
	return nil
}

func (*moderationUsersAPIRepository) ApplyModerationAutoBan(context.Context, int64) (service.ModerationAutoBanResult, error) {
	return service.ModerationAutoBanResult{}, nil
}

func (*moderationUsersAPIRepository) ApplyModerationHit(context.Context, service.ModerationHit) (service.ModerationHitResult, error) {
	return service.ModerationHitResult{}, nil
}

func TestContentModerationUsersAPICanLookupAndConfigureUnseenUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &moderationUsersAPIRepository{state: service.ModerationUserState{UserID: 27, Status: service.StatusActive}}
	svc := service.NewContentModerationService(moderationUsersAPISettings{}, repo, nil, nil, nil, nil, nil, nil)
	h := NewContentModerationHandler(svc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
		c.Next()
	})
	router.GET("/risk-control/users", h.ListModerationUsers)
	router.POST("/risk-control/users/:user_id/actions", h.ManageUser)

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/risk-control/users?page=1&page_size=20&state=all&user_id=27", nil))
	require.Equal(t, http.StatusOK, listResponse.Code, listResponse.Body.String())
	require.Equal(t, int64(27), repo.listedUserID)
	require.Contains(t, listResponse.Body.String(), `"user_id":27`)
	require.Contains(t, listResponse.Body.String(), `"version":0`)

	actionResponse := httptest.NewRecorder()
	body := `{"action":"exemption","version":0,"auto_ban_exempt":true,"reason":"pre-approved account","confirm":true}`
	req := httptest.NewRequest(http.MethodPost, "/risk-control/users/27/actions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(actionResponse, req)
	require.Equal(t, http.StatusOK, actionResponse.Code, actionResponse.Body.String())
	require.Equal(t, int64(9), repo.lastAction.ActorID)
	require.Equal(t, int64(0), repo.lastAction.Version)
	require.True(t, repo.state.AutoBanExempt)
	require.Equal(t, int64(1), repo.state.Version)
	require.Equal(t, "pre-approved account", repo.state.Reason)
}
