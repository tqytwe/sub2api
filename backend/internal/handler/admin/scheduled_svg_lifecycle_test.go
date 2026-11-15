package admin

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type svgPlanLifecycleRepo struct {
	service.ScheduledTestPlanRepository
	plan    *service.ScheduledTestPlan
	deleted bool
}

func (r *svgPlanLifecycleRepo) GetByID(context.Context, int64) (*service.ScheduledTestPlan, error) {
	copy := *r.plan
	return &copy, nil
}
func (r *svgPlanLifecycleRepo) UpdateWithVersion(_ context.Context, plan *service.ScheduledTestPlan, _ time.Time) (*service.ScheduledTestPlan, error) {
	r.plan = plan
	return plan, nil
}
func (r *svgPlanLifecycleRepo) Delete(context.Context, int64) error { r.deleted = true; return nil }

func TestScheduledSVGPlanCanBeDeletedWithoutQualityOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &svgPlanLifecycleRepo{plan: &service.ScheduledTestPlan{ID: 3, PelicanConfig: &service.PelicanTestConfig{QuestionKind: "pelican"}}}
	h := NewScheduledTestHandler(service.NewScheduledTestService(repo, nil))
	router := gin.New()
	router.DELETE("/plans/:id", h.Delete)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("DELETE", "/plans/3", nil))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.True(t, repo.deleted)
}

func TestScheduledSVGPlanExplicitNullDisablesDrawingWhileOmissionPreservesIt(t *testing.T) {
	for _, config := range []string{`,"pelican_config":null`, ``} {
		t.Run(config, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			version := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
			repo := &svgPlanLifecycleRepo{plan: &service.ScheduledTestPlan{ID: 3, UpdatedAt: version, CronExpression: "*/30 * * * *", PelicanConfig: &service.PelicanTestConfig{QuestionKind: "pelican", Prompt: "draw"}}}
			h := NewScheduledTestHandler(service.NewScheduledTestService(repo, nil))
			router := gin.New()
			router.PUT("/plans/:id", h.Update)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/plans/3", strings.NewReader(`{"enabled":false,"expected_updated_at":"2026-11-14T12:00:00Z"`+config+`}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, 200, rec.Code, rec.Body.String())
			var saved service.ScheduledTestPlan
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &saved))
			if config != "" {
				require.Nil(t, saved.PelicanConfig)
			} else {
				require.Equal(t, "pelican", saved.PelicanConfig.QuestionKind)
			}
		})
	}
}
