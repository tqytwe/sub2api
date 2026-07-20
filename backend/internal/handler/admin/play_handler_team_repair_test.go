package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type handlerTeamRepairRepo struct {
	service.PlayRepository
	membership *service.PlayTeamMembershipDB
}

type handlerTeamRepairUserRepo struct {
	service.UserRepository
}

func (handlerTeamRepairUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	return &service.User{ID: 99, TotpEnabled: true}, nil
}

func (handlerTeamRepairUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

type handlerTeamRepairTotpCache struct {
	service.TotpCache
}

func (handlerTeamRepairTotpCache) HasStepUpGrant(context.Context, int64, string) (bool, error) {
	return true, nil
}

type handlerTeamRepairAuditRepo struct {
	mu   sync.Mutex
	logs []*service.AuditLog
}

func (r *handlerTeamRepairAuditRepo) BatchInsert(_ context.Context, logs []*service.AuditLog) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, logs...)
	return int64(len(logs)), nil
}

func (r *handlerTeamRepairAuditRepo) Insert(_ context.Context, log *service.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, log)
	return nil
}

func (*handlerTeamRepairAuditRepo) List(context.Context, *service.AuditLogFilter) (*service.AuditLogList, error) {
	return &service.AuditLogList{}, nil
}

func (*handlerTeamRepairAuditRepo) GetByID(context.Context, int64) (*service.AuditLog, error) {
	return nil, service.ErrAuditLogNotFound
}

func (*handlerTeamRepairAuditRepo) Count(context.Context) (int64, error) {
	return 0, nil
}

func (*handlerTeamRepairAuditRepo) TruncateAll(context.Context) error {
	return nil
}

func (*handlerTeamRepairAuditRepo) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (r *handlerTeamRepairRepo) LockAdminTeamCandidateUser(context.Context, int64) (*service.PlayAdminTeamMemberCandidate, error) {
	return &service.PlayAdminTeamMemberCandidate{
		UserID:      42,
		Email:       "member@example.com",
		DisplayName: "member",
		Status:      service.StatusActive,
	}, nil
}

func (r *handlerTeamRepairRepo) GetActiveTeamMembership(context.Context, int64) (*service.PlayTeamMembershipDB, error) {
	return r.membership, nil
}

func (r *handlerTeamRepairRepo) LockActiveTeamMembership(context.Context, int64) (*service.PlayTeamMembershipDB, error) {
	return r.membership, nil
}

func (r *handlerTeamRepairRepo) LockTeamForAdmin(context.Context, int64) (*service.PlayTeamDB, error) {
	return &service.PlayTeamDB{ID: 9, Name: "Target", CaptainUserID: 7}, nil
}

func (r *handlerTeamRepairRepo) HasTeamRewardSnapshotAt(context.Context, []int64, time.Time) (bool, error) {
	return false, nil
}

func (r *handlerTeamRepairRepo) HasTeamMembershipOverlap(context.Context, int64, time.Time, int64) (bool, error) {
	return false, nil
}

func (r *handlerTeamRepairRepo) JoinTeamAt(context.Context, int64, int64, time.Time) error {
	return nil
}

func (r *handlerTeamRepairRepo) InsertTeamEvent(context.Context, service.PlayTeamEvent) error {
	return nil
}

func setupAdminTeamRepairHandler(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	playService := service.NewPlayService(&handlerTeamRepairRepo{}, nil, nil, nil, nil, client)
	handler := NewAdminPlayHandler(playService, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Set("auth_method", service.AuditAuthMethodJWT)
		c.Next()
	})
	router.POST("/api/v1/admin/play/teams/:id/members", handler.AddOrMoveTeamMember)
	return router, mock
}

func postAdminTeamRepair(t *testing.T, router *gin.Engine, body map[string]any, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/play/teams/9/members", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAdminTeamRepairRequiresIdempotencyKeyAndValidReason(t *testing.T) {
	router, _ := setupAdminTeamRepairHandler(t)

	rec := postAdminTeamRepair(t, router, map[string]any{
		"user_id":   42,
		"operation": "add",
		"reason":    "repair missing membership",
	}, "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"reason":"IDEMPOTENCY_KEY_REQUIRED"`)

	rec = postAdminTeamRepair(t, router, map[string]any{
		"user_id":   42,
		"operation": "add",
		"reason":    "short",
	}, "repair-key-1")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"reason":"PLAY_TEAM_ADMIN_REASON_INVALID"`)

	rec = postAdminTeamRepair(t, router, map[string]any{
		"user_id":   42,
		"operation": "add",
		"reason":    "repair missing membership",
	}, strings.Repeat("x", 129))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"reason":"IDEMPOTENCY_KEY_INVALID"`)
}

func TestAdminTeamRepairImmediateAddSkipsStepUpAndSetsAuditAction(t *testing.T) {
	router, mock := setupAdminTeamRepairHandler(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	rec := postAdminTeamRepair(t, router, map[string]any{
		"user_id":   42,
		"operation": "add",
		"reason":    "repair missing membership",
	}, "repair-key-2")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"added"`)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminTeamRepairMoveAndBackdateAlwaysRequireJWTStepUp(t *testing.T) {
	router, _ := setupAdminTeamRepairHandler(t)

	rec := postAdminTeamRepair(t, router, map[string]any{
		"user_id":                 42,
		"operation":               "move",
		"reason":                  "move member to correct team",
		"expected_source_team_id": 8,
	}, "repair-key-3")
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = postAdminTeamRepair(t, router, map[string]any{
		"user_id":      42,
		"operation":    "add",
		"effective_at": "2026-07-10T08:00:00+08:00",
		"reason":       "backfill current month membership",
	}, "repair-key-4")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminTeamRepairMoveRejectsAdminAPIKeyEvenWhenStepUpFeatureIsOff(t *testing.T) {
	router := gin.New()
	playService := service.NewPlayService(&handlerTeamRepairRepo{}, nil, nil, nil, nil, nil)
	handler := NewAdminPlayHandler(playService, nil, nil)
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Set("auth_method", service.AuditAuthMethodAdminAPIKey)
		c.Next()
	})
	router.POST("/api/v1/admin/play/teams/:id/members", handler.AddOrMoveTeamMember)

	rec := postAdminTeamRepair(t, router, map[string]any{
		"user_id":                 42,
		"operation":               "move",
		"reason":                  "move member to correct team",
		"expected_source_team_id": 8,
	}, "repair-key-5")

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"code":"STEP_UP_ADMIN_API_KEY_FORBIDDEN"`)
}

func TestAdminTeamRepairDeniedMoveKeepsExplicitAuditAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auditRepo := &handlerTeamRepairAuditRepo{}
	auditService := service.NewAuditLogService(auditRepo, nil)
	auditService.Start()
	handler := NewAdminPlayHandler(service.NewPlayService(&handlerTeamRepairRepo{}, nil, nil, nil, nil, nil), nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		c.Set("auth_method", service.AuditAuthMethodAdminAPIKey)
		c.Next()
	})
	router.Use(gin.HandlerFunc(middleware.NewAuditLogMiddleware(auditService)))
	router.POST("/api/v1/admin/play/teams/:id/members", handler.AddOrMoveTeamMember)

	rec := postAdminTeamRepair(t, router, map[string]any{
		"user_id":                 42,
		"operation":               "move",
		"reason":                  "move member to correct team",
		"expected_source_team_id": 8,
	}, "repair-key-denied-audit")
	require.Equal(t, http.StatusForbidden, rec.Code)
	auditService.Stop()

	auditRepo.mu.Lock()
	logs := append([]*service.AuditLog(nil), auditRepo.logs...)
	auditRepo.mu.Unlock()
	require.Len(t, logs, 1)
	require.Equal(t, service.AuditActionAdminPlayTeamMemberMove, logs[0].Action)
	require.Equal(t, http.StatusForbidden, logs[0].StatusCode)
}

func TestAdminTeamRepairBackdateExecutesWithJWTStepUpGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	playService := service.NewPlayService(&handlerTeamRepairRepo{}, nil, nil, nil, nil, client)
	totpService := service.NewTotpService(nil, nil, handlerTeamRepairTotpCache{}, nil, nil, nil)
	userService := service.NewUserService(handlerTeamRepairUserRepo{}, nil, nil, nil)
	handler := NewAdminPlayHandler(playService, totpService, userService)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Set("auth_method", service.AuditAuthMethodJWT)
		c.Next()
	})
	router.POST("/api/v1/admin/play/teams/:id/members", handler.AddOrMoveTeamMember)
	mock.ExpectBegin()
	mock.ExpectCommit()

	rec := postAdminTeamRepair(t, router, map[string]any{
		"user_id":      42,
		"operation":    "add",
		"effective_at": "2026-07-10T08:00:00+08:00",
		"reason":       "backfill current month membership",
	}, "repair-key-step-up-success")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"added"`)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminTeamRepairMoveAlreadyInTargetRejectsMissingExpectedSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	repo := &handlerTeamRepairRepo{
		membership: &service.PlayTeamMembershipDB{
			ID:       5,
			TeamID:   9,
			UserID:   42,
			JoinedAt: time.Date(2026, time.July, 20, 8, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60)),
		},
	}
	playService := service.NewPlayService(repo, nil, nil, nil, nil, client)
	totpService := service.NewTotpService(nil, nil, handlerTeamRepairTotpCache{}, nil, nil, nil)
	userService := service.NewUserService(handlerTeamRepairUserRepo{}, nil, nil, nil)
	handler := NewAdminPlayHandler(playService, totpService, userService)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Set("auth_method", service.AuditAuthMethodJWT)
		c.Next()
	})
	router.POST("/api/v1/admin/play/teams/:id/members", handler.AddOrMoveTeamMember)

	rec := postAdminTeamRepair(t, router, map[string]any{
		"user_id":   42,
		"operation": "move",
		"reason":    "confirm existing target membership",
	}, "repair-key-no-op-move")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"reason":"PLAY_TEAM_MEMBER_SOURCE_REQUIRED"`)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminTeamMemberAuditActionDistinguishesAddAndMove(t *testing.T) {
	require.Equal(t, service.AuditActionAdminPlayTeamMemberAdd, adminTeamMemberAuditAction(service.AdminTeamMemberOperationAdd))
	require.Equal(t, service.AuditActionAdminPlayTeamMemberMove, adminTeamMemberAuditAction(service.AdminTeamMemberOperationMove))
}

func TestAdminTeamRepairReplaysSameIdempotencyKeyWithoutSecondMutation(t *testing.T) {
	repo := newMemoryIdempotencyRepoStub()
	cfg := service.DefaultIdempotencyConfig()
	cfg.ObserveOnly = false
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(repo, cfg))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(nil)
	})

	router, mock := setupAdminTeamRepairHandler(t)
	mock.ExpectBegin()
	mock.ExpectCommit()
	body := map[string]any{
		"user_id":   42,
		"operation": "add",
		"reason":    "repair missing membership",
	}

	first := postAdminTeamRepair(t, router, body, "repair-replay-key")
	second := postAdminTeamRepair(t, router, body, "repair-replay-key")

	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.JSONEq(t, first.Body.String(), second.Body.String())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminTeamRepairPersistsStableAuditActionAndSafeBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	auditRepo := &handlerTeamRepairAuditRepo{}
	auditService := service.NewAuditLogService(auditRepo, nil)
	auditService.Start()
	playService := service.NewPlayService(&handlerTeamRepairRepo{}, nil, nil, nil, nil, client)
	handler := NewAdminPlayHandler(playService, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		c.Set("auth_method", service.AuditAuthMethodJWT)
		c.Next()
	})
	router.Use(gin.HandlerFunc(middleware.NewAuditLogMiddleware(auditService)))
	router.POST("/api/v1/admin/play/teams/:id/members", handler.AddOrMoveTeamMember)
	mock.ExpectBegin()
	mock.ExpectCommit()

	rec := postAdminTeamRepair(t, router, map[string]any{
		"user_id":   42,
		"operation": "add",
		"reason":    "repair missing membership token=sk-audit-secret-123456 bearer ABCDEFGHIJKLMNOPQRSTUVWXYZ",
	}, "audit-idempotency-secret")
	require.Equal(t, http.StatusOK, rec.Code)
	auditService.Stop()

	auditRepo.mu.Lock()
	logs := append([]*service.AuditLog(nil), auditRepo.logs...)
	auditRepo.mu.Unlock()
	require.Len(t, logs, 1)
	require.Equal(t, service.AuditActionAdminPlayTeamMemberAdd, logs[0].Action)
	require.Equal(t, http.StatusOK, logs[0].StatusCode)
	require.NotNil(t, logs[0].ActorUserID)
	require.Equal(t, int64(99), *logs[0].ActorUserID)
	require.Equal(t, "<credential-bearing body omitted>", logs[0].RequestBody)
	require.NotContains(t, logs[0].RequestBody, "sk-audit-secret-123456")
	require.NotContains(t, logs[0].RequestBody, "audit-idempotency-secret")
	require.NotContains(t, logs[0].RequestBody, "invite_code")
	require.NotContains(t, logs[0].RequestBody, "token")
	require.Equal(t, int64(9), logs[0].Extra["target_team_id"])
	require.Equal(t, int64(42), logs[0].Extra["target_user_id"])
	require.Equal(t, service.AdminTeamMemberOperationAdd, logs[0].Extra["operation"])
	require.Equal(t, service.PlayTeamEventReasonAdminManualMembershipRepair, logs[0].Extra["reason_code"])
	require.NotContains(t, logs[0].Extra, "repair_reason")
	require.NoError(t, mock.ExpectationsWereMet())
}
