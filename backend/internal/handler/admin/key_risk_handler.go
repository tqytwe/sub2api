package admin

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/keyrisk"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// keyRiskAPIKeys evicts the normal auth caches after the atomic restriction commits.
type keyRiskAPIKeys interface {
	InvalidateAuthCacheByKey(context.Context, string)
}

func (h *SettingHandler) SetKeyRiskAPIKeys(keys *service.APIKeyService) {
	h.keyRiskKeys = keys
}

func (h *SettingHandler) StartKeyRisk(dsn string) (func(), error) {
	h.keyRiskMu.Lock()
	if h.keyRiskStop != nil {
		h.keyRiskMu.Unlock()
		return h.StopKeyRisk, nil
	}
	db, e := sql.Open("postgres", dsn)
	if e != nil {
		h.keyRiskMu.Unlock()
		return nil, e
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	service := &keyrisk.Service{DB: db, Mail: h.emailService}
	service.Start()
	h.keyRisk = service
	h.keyRiskStop = func() {
		service.Stop()
		_ = db.Close()
	}
	h.keyRiskMu.Unlock()
	return h.StopKeyRisk, nil
}

// StopKeyRisk stops the scanner and closes its dedicated database pool. It is
// safe to call more than once, which lets application cleanup and tests share
// the same shutdown path.
func (h *SettingHandler) StopKeyRisk() {
	h.keyRiskMu.Lock()
	stop := h.keyRiskStop
	h.keyRiskStop = nil
	h.keyRisk = nil
	h.keyRiskMu.Unlock()
	if stop != nil {
		stop()
	}
}
func (h *SettingHandler) keyRiskReady(c *gin.Context) bool {
	if !keyRiskAdministrator(c) {
		return false
	}
	if h.keyRisk == nil {
		response.Error(c, 503, "Key risk service unavailable")
		return false
	}
	return true
}

func keyRiskAdministrator(c *gin.Context) bool {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	role, _ := middleware.GetUserRoleFromContext(c)
	if !ok || subject.UserID <= 0 || role != service.RoleAdmin {
		response.Forbidden(c, "Administrator access required")
		return false
	}
	return true
}
func (h *SettingHandler) GetKeyRisk(c *gin.Context) {
	if !h.keyRiskReady(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	before, e := strconv.ParseInt(c.DefaultQuery("before", "0"), 10, 64)
	if e != nil || before < 0 {
		response.BadRequest(c, "Invalid cursor")
		return
	}
	cfg, e := h.keyRisk.Config(ctx)
	if e != nil {
		response.Error(c, 503, "Key risk configuration unavailable")
		return
	}
	events, e := h.keyRisk.Events(ctx, before)
	if e != nil {
		response.Error(c, 503, "Key risk records unavailable")
		return
	}
	var at sql.NullTime
	var issue string
	e = h.keyRisk.DB.QueryRowContext(ctx, `SELECT last_scan_at,last_error FROM key_risk_control WHERE singleton`).Scan(&at, &issue)
	if e != nil {
		response.Error(c, 503, "Key risk status unavailable")
		return
	}
	var pending, uncertain, sent int
	e = h.keyRisk.DB.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE state='pending'),count(*) FILTER(WHERE state='uncertain'),count(*) FILTER(WHERE state='sent') FROM key_risk_outbox`).Scan(&pending, &uncertain, &sent)
	if e != nil {
		response.Error(c, 503, "Key risk delivery status unavailable")
		return
	}
	response.Success(c, gin.H{"config": cfg, "items": events, "last_scan_at": at.Time, "coverage_issue": issue, "pending": pending, "uncertain": uncertain, "sent": sent})
}
func (h *SettingHandler) SaveKeyRisk(c *gin.Context) {
	if !h.keyRiskReady(c) {
		return
	}
	cfg := keyrisk.DefaultConfig()
	if e := c.ShouldBindJSON(&cfg); e != nil || cfg.Validate() != nil {
		response.BadRequest(c, "Invalid key risk configuration")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if e := h.keyRisk.Save(ctx, cfg); e != nil {
		response.Error(c, 503, "Unable to save key risk configuration")
		return
	}
	middleware.SetAuditAction(c, "admin.key_risk.configure")
	response.Success(c, cfg)
}
func (h *SettingHandler) ActOnKeyRisk(c *gin.Context) {
	if !h.keyRiskReady(c) {
		return
	}
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Action          string   `json:"action"`
		Generation      int64    `json:"generation"`
		Confirm         bool     `json:"confirm"`
		AllowedNetworks []string `json:"allowed_networks"`
	}
	if e != nil || id <= 0 || c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "Invalid risk action")
		return
	}
	switch req.Action {
	case "notify", "ignored", "resolved", "pending", "restrict":
	default:
		response.BadRequest(c, "Invalid risk action")
		return
	}
	actor, _ := middleware.GetAuthSubjectFromContext(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	queued := 0
	switch req.Action {
	case "restrict":
		if !req.Confirm || req.Generation <= 0 {
			response.BadRequest(c, "Explicit confirmation and current generation required")
			return
		}
		networks, err := keyRiskAllowedNetworks(req.AllowedNetworks)
		if err != nil {
			response.BadRequest(c, "Supply 1 to 256 allowed IPs or CIDRs")
			return
		}
		if h.keyRiskKeys == nil {
			response.Error(c, 503, "API key service unavailable")
			return
		}
		var key string
		key, e = h.keyRisk.RestrictIPWhitelist(ctx, id, req.Generation, networks)
		if e == nil {
			// The committed ACL must reach auth caches even if the request ends.
			cacheCtx, cancelCache := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
			h.keyRiskKeys.InvalidateAuthCacheByKey(cacheCtx, key)
			cancelCache()
		}
	case "notify":
		queued, e = h.keyRisk.NotifyUsers(ctx, id, actor.UserID)
	default:
		e = h.keyRisk.SetState(ctx, id, actor.UserID, req.Action)
	}
	if e != nil {
		response.Error(c, 409, "Risk action unavailable; refresh records")
		return
	}
	middleware.SetAuditAction(c, "admin.key_risk."+req.Action)
	response.Success(c, gin.H{"queued": queued > 0, "recipients": queued})
}

// A nonempty whitelist enables existing per-key admission, independent of the
// global IP guard's enabled/observation switches. No IP-wide blacklist is changed.
func keyRiskAllowedNetworks(raw []string) ([]string, error) {
	if len(raw) == 0 || len(raw) > 256 {
		return nil, fmt.Errorf("invalid allowed networks")
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, value := range raw {
		network, err := normalizeKeyRiskNetwork(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}
		normalized := network.String()
		if !seen[normalized] {
			out = append(out, normalized)
			seen[normalized] = true
		}
	}
	return out, nil
}

func normalizeKeyRiskNetwork(value string) (netip.Prefix, error) {
	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return netip.Prefix{}, err
		}
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil || addr.Zone() != "" {
		if err == nil {
			err = fmt.Errorf("IP zones are not supported")
		}
		return netip.Prefix{}, err
	}
	addr = addr.Unmap()
	bits := 128
	if addr.Is4() {
		bits = 32
	}
	return netip.PrefixFrom(addr, bits), nil
}
