package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type modelBillingHandlerRepo struct {
	value, revision string
}

func (r *modelBillingHandlerRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (r *modelBillingHandlerRepo) GetValue(context.Context, string) (string, error) {
	return r.value, nil
}
func (r *modelBillingHandlerRepo) Set(context.Context, string, string) error { return nil }
func (r *modelBillingHandlerRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *modelBillingHandlerRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (r *modelBillingHandlerRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *modelBillingHandlerRepo) Delete(context.Context, string) error { return nil }
func (r *modelBillingHandlerRepo) GetValueWithRevision(context.Context, string) (string, string, error) {
	return r.value, r.revision, nil
}
func (r *modelBillingHandlerRepo) CompareAndSet(_ context.Context, _, expectedRevision, expectedValue, value string) (string, error) {
	if expectedRevision != r.revision || expectedValue != r.value {
		return "", service.ErrModelBillingRevisionConflict
	}
	r.value, r.revision = value, r.revision+"-next"
	return r.revision, nil
}

func TestModelBillingHandlerRequiresRevisionEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &modelBillingHandlerRepo{value: `{"enabled":false,"rules":[]}`, revision: "r1"}
	h := NewSettingHandler(service.NewSettingService(repo, nil), nil, nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-billing/config", strings.NewReader(`{"config":{"enabled":false,"rules":[]}}`))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	h.SaveModelBillingConfig(c)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestModelBillingHandlerReturnsRevisionConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &modelBillingHandlerRepo{value: `{"enabled":false,"rules":[]}`, revision: "r2"}
	h := NewSettingHandler(service.NewSettingService(repo, nil), nil, nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-billing/config", strings.NewReader(`{"revision":"r1","config":{"enabled":false,"rules":[]}}`))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	h.SaveModelBillingConfig(c)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
}

func TestModelBillingHandlerPreservesOmittedLegacyScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{
		`{"revision":"r1","config":{"enabled":true,"rules":[{"model":"gpt-5","multiplier":3}]}}`,
		`{"revision":"r1","config":{"enabled":true,"rules":[{"model":"new-model","multiplier":3}]}}`,
	} {
		repo := &modelBillingHandlerRepo{value: `{"enabled":true,"rules":[{"model":"gpt-5","multiplier":2,"group_ids":[7]}]}`, revision: "r1"}
		svc := service.NewSettingService(repo, nil)
		svc.SetModelBillingGroupReader(modelBillingHandlerGroups{})
		h := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-billing/config", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.SaveModelBillingConfig(c)
		if strings.Contains(body, "new-model") {
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("new model without explicit scope: status=%d", rec.Code)
			}
		} else {
			if rec.Code != http.StatusOK || !strings.Contains(repo.value, `"group_ids":[7]`) {
				t.Fatalf("legacy save widened scope: status=%d stored=%s", rec.Code, repo.value)
			}
		}
	}
}

type modelBillingHandlerGroups struct{}

func (modelBillingHandlerGroups) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	if id == 7 {
		return &service.Group{ID: 7, Status: service.StatusActive}, nil
	}
	if id == 8 {
		return &service.Group{ID: 8, Status: service.StatusDisabled}, nil
	}
	return nil, service.ErrGroupNotFound
}

func TestModelBillingHandlerValidationIsClientError(t *testing.T) {
	for _, config := range []string{
		`{"enabled":true,"rules":[{"model":"gpt-5","multiplier":0}]}`,
		`{"enabled":true,"rules":[{"model":"gpt-5","multiplier":2,"group_ids":[999]}]}`,
		`{"enabled":true,"rules":[{"model":"gpt-5","multiplier":2,"group_ids":[8]}]}`,
	} {
		repo := &modelBillingHandlerRepo{value: `{"enabled":false,"rules":[]}`, revision: "r1"}
		svc := service.NewSettingService(repo, nil)
		svc.SetModelBillingGroupReader(modelBillingHandlerGroups{})
		h := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-billing/config", strings.NewReader(`{"revision":"r1","config":`+config+`}`))
		c.Request.Header.Set("Content-Type", "application/json")
		h.SaveModelBillingConfig(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("validation status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
}

func TestModelBillingHandlerAllowsExplicitGlobalScope(t *testing.T) {
	repo := &modelBillingHandlerRepo{value: `{"enabled":true,"rules":[{"model":"gpt-5","multiplier":2,"group_ids":[7]}]}`, revision: "r1"}
	svc := service.NewSettingService(repo, nil)
	h := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-billing/config", strings.NewReader(`{"revision":"r1","config":{"enabled":true,"rules":[{"model":"gpt-5","multiplier":3,"group_ids":[]}]}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.SaveModelBillingConfig(c)
	if rec.Code != http.StatusOK || strings.Contains(repo.value, `[7]`) {
		t.Fatalf("explicit global scope must be allowed: %d %s", rec.Code, repo.value)
	}
}
