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
