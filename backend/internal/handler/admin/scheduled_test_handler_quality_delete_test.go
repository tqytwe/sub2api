package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type scheduledPlanRepositoryStub struct {
	plan       *service.ScheduledTestPlan
	deleteCall bool
}

func (s *scheduledPlanRepositoryStub) Create(context.Context, *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	return nil, nil
}

func (s *scheduledPlanRepositoryStub) GetByID(context.Context, int64) (*service.ScheduledTestPlan, error) {
	return s.plan, nil
}

func (s *scheduledPlanRepositoryStub) ListByAccountID(context.Context, int64) ([]*service.ScheduledTestPlan, error) {
	return nil, nil
}

func (s *scheduledPlanRepositoryStub) ListQualityPlans(context.Context) ([]*service.ScheduledTestPlan, error) {
	return nil, nil
}

func (s *scheduledPlanRepositoryStub) TriggerQuality(context.Context, int64) error {
	return nil
}

func (s *scheduledPlanRepositoryStub) ListDue(context.Context, time.Time) ([]*service.ScheduledTestPlan, error) {
	return nil, nil
}

func (s *scheduledPlanRepositoryStub) Update(context.Context, *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	return nil, nil
}

func (s *scheduledPlanRepositoryStub) Delete(context.Context, int64) error {
	s.deleteCall = true
	return nil
}

func (s *scheduledPlanRepositoryStub) UpdateAfterRun(context.Context, int64, time.Time, time.Time) error {
	return nil
}

func (s *scheduledPlanRepositoryStub) QualityOwnership(context.Context, int64, *service.QualityOwnershipRequest) (*service.QualityOwnershipPreview, error) {
	return nil, nil
}

func TestGenericScheduledPlanDeleteRejectsQualityRuleBeforeRepositoryDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	planRepo := &scheduledPlanRepositoryStub{plan: &service.ScheduledTestPlan{
		ID: 7, AccountID: 9, ModelID: "gpt-test", CronExpression: "*/30 * * * *", Enabled: true,
		PelicanConfig: &service.PelicanTestConfig{Quality: &service.QualityPolicy{Action: "disable_scheduling"}},
	}}
	scheduledSvc := service.NewScheduledTestService(planRepo, nil)
	handler := NewScheduledTestHandler(scheduledSvc)
	router := gin.New()
	router.DELETE("/admin/scheduled-test-plans/:id", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/admin/scheduled-test-plans/7", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	require.Equal(t, http.StatusConflict, response.Code)
	require.Contains(t, response.Body.String(), "QUALITY_OWNERSHIP_REQUIRED")
	require.False(t, planRepo.deleteCall)
}
