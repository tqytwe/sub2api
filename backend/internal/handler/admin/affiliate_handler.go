package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AffiliateHandler handles admin affiliate (邀请返利) management:
// listing users with custom settings, updating per-user invite codes
// and exclusive rebate rates, and batch operations.
type AffiliateHandler struct {
	affiliateService *service.AffiliateService
	adminService     service.AdminService
}

// NewAffiliateHandler creates a new admin affiliate handler.
func NewAffiliateHandler(affiliateService *service.AffiliateService, adminService service.AdminService) *AffiliateHandler {
	return &AffiliateHandler{
		affiliateService: affiliateService,
		adminService:     adminService,
	}
}

// ListUsers returns paginated users with custom affiliate settings.
// GET /api/v1/admin/affiliates/users
func (h *AffiliateHandler) ListUsers(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := c.Query("search")

	entries, total, err := h.affiliateService.AdminListCustomUsers(c.Request.Context(), service.AffiliateAdminFilter{
		Search:   search,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, entries, total, page, pageSize)
}

// UpdateUserSettings updates a user's affiliate settings.
// PUT /api/v1/admin/affiliates/users/:user_id
//
// Both fields are optional and applied independently.
type UpdateAffiliateUserRequest struct {
	AffCode              *string  `json:"aff_code"`
	AffRebateRatePercent *float64 `json:"aff_rebate_rate_percent"`
	// ClearRebateRate explicitly clears the per-user rate (sets it to NULL).
	// Used to disambiguate from "field not provided".
	ClearRebateRate bool `json:"clear_rebate_rate"`
}

func (h *AffiliateHandler) UpdateUserSettings(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}

	var req UpdateAffiliateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.AffCode != nil {
		if err := h.affiliateService.AdminUpdateUserAffCode(c.Request.Context(), userID, *req.AffCode); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	if req.ClearRebateRate {
		if err := h.affiliateService.AdminSetUserRebateRate(c.Request.Context(), userID, nil); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	} else if req.AffRebateRatePercent != nil {
		if err := h.affiliateService.AdminSetUserRebateRate(c.Request.Context(), userID, req.AffRebateRatePercent); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	response.Success(c, gin.H{"user_id": userID})
}

// ClearUserSettings removes ALL of a user's custom affiliate settings — clears
// the exclusive rebate rate AND regenerates the invite code as a new system
// random one. Conceptually this "removes the user from the custom list".
//
// Both writes happen in this handler; failure of one leaves the other applied,
// but the operation is idempotent so the admin can re-run it safely.
// DELETE /api/v1/admin/affiliates/users/:user_id
func (h *AffiliateHandler) ClearUserSettings(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	if err := h.affiliateService.AdminSetUserRebateRate(c.Request.Context(), userID, nil); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if _, err := h.affiliateService.AdminResetUserAffCode(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"user_id": userID})
}

// BatchSetRate applies the same rebate rate (or clears it) to multiple users.
//
// Protocol: pass `clear: true` to clear rates (aff_rebate_rate_percent is
// ignored). Otherwise aff_rebate_rate_percent is required and applied to
// every user_id. The explicit `clear` flag exists because Go's JSON unmarshal
// can't distinguish a missing field from `null`, and a silent clear from a
// frontend that forgot to include the rate would be a footgun.
//
// POST /api/v1/admin/affiliates/users/batch-rate
type BatchSetRateRequest struct {
	UserIDs              []int64  `json:"user_ids" binding:"required"`
	AffRebateRatePercent *float64 `json:"aff_rebate_rate_percent"`
	Clear                bool     `json:"clear"`
}

func (h *AffiliateHandler) BatchSetRate(c *gin.Context) {
	var req BatchSetRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.UserIDs) == 0 {
		response.BadRequest(c, "user_ids cannot be empty")
		return
	}
	if !req.Clear && req.AffRebateRatePercent == nil {
		response.BadRequest(c, "aff_rebate_rate_percent is required unless clear=true")
		return
	}
	rate := req.AffRebateRatePercent
	if req.Clear {
		rate = nil
	}
	if err := h.affiliateService.AdminBatchSetUserRebateRate(c.Request.Context(), req.UserIDs, rate); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"affected": len(req.UserIDs)})
}

// AffiliateUserSummary is the minimal user shape returned by LookupUsers,
// shared with the frontend's add-custom-user picker.
type AffiliateUserSummary struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// LookupUsers searches users by email/username for the "add custom user" modal.
// GET /api/v1/admin/affiliates/users/lookup?q=
func (h *AffiliateHandler) LookupUsers(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		response.Success(c, []AffiliateUserSummary{})
		return
	}
	users, _, err := h.adminService.ListUsers(c.Request.Context(), 1, 20, service.UserListFilters{Search: keyword}, "email", "asc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result := make([]AffiliateUserSummary, len(users))
	for i, u := range users {
		result[i] = AffiliateUserSummary{ID: u.ID, Email: u.Email, Username: u.Username}
	}
	response.Success(c, result)
}

// GetUserOverview returns one user's affiliate overview.
// GET /api/v1/admin/affiliates/users/:user_id/overview
func (h *AffiliateHandler) GetUserOverview(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	overview, err := h.affiliateService.AdminGetUserOverview(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

// ListInviteRecords returns all inviter-invitee relationships.
// GET /api/v1/admin/affiliates/invites
func (h *AffiliateHandler) ListInviteRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := parseAffiliateRecordFilter(c, page, pageSize)
	items, total, err := h.affiliateService.AdminListInviteRecords(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

// ListRebateRecords returns all order-level affiliate rebate records.
// GET /api/v1/admin/affiliates/rebates
func (h *AffiliateHandler) ListRebateRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := parseAffiliateRecordFilter(c, page, pageSize)
	items, total, err := h.affiliateService.AdminListRebateRecords(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

// ListTransferRecords returns all affiliate quota-to-balance transfer records.
// GET /api/v1/admin/affiliates/transfers
func (h *AffiliateHandler) ListTransferRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := parseAffiliateRecordFilter(c, page, pageSize)
	items, total, err := h.affiliateService.AdminListTransferRecords(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func parseAffiliateRecordFilter(c *gin.Context, page, pageSize int) service.AffiliateRecordFilter {
	filter := service.AffiliateRecordFilter{
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
		SortBy:   c.Query("sort_by"),
		SortDesc: c.Query("sort_order") != "asc",
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	userTZ := c.Query("timezone")
	if t := parseAffiliateRecordStartTime(c.Query("start_at"), userTZ); t != nil {
		filter.StartAt = t
	}
	if t := parseAffiliateRecordEndTime(c.Query("end_at"), userTZ); t != nil {
		filter.EndAt = t
	}
	return filter
}

func parseAffiliateRecordStartTime(raw string, userTZ string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed
	}
	if parsed, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ); err == nil {
		return &parsed
	}
	return nil
}

func parseAffiliateRecordEndTime(raw string, userTZ string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed
	}
	if parsed, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ); err == nil {
		end := parsed.AddDate(0, 0, 1).Add(-time.Nanosecond)
		return &end
	}
	return nil
}

func (h *AffiliateHandler) referralCampaignService(c *gin.Context) (*service.ReferralCampaignService, bool) {
	if h == nil || h.affiliateService == nil || h.affiliateService.ReferralCampaign() == nil {
		response.BadRequest(c, "referral campaign service unavailable")
		return nil, false
	}
	return h.affiliateService.ReferralCampaign(), true
}

func (h *AffiliateHandler) GetReferralCampaign(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	campaign, err := svc.GetCampaignDetail(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

type referralCampaignUpsertRequest struct {
	service.ReferralCampaign
	ExpectedVersion int64                          `json:"expected_version"`
	Tiers           []service.ReferralCampaignTier `json:"tiers" binding:"required"`
}

func (h *AffiliateHandler) ListReferralCampaigns(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := svc.ListCampaigns(c.Request.Context(), service.ReferralCampaignListFilter{Page: page, PageSize: pageSize, Status: c.Query("status"), Search: c.Query("search")})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AffiliateHandler) CreateReferralCampaign(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	var req referralCampaignUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	req.ID = 0
	req.CreatedBy = getAdminIDFromContext(c)
	req.ApprovedBy = nil
	created, err := svc.CreateCampaign(c.Request.Context(), req.ReferralCampaign, req.Tiers)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, created)
}

func (h *AffiliateHandler) UpdateReferralCampaign(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	var req referralCampaignUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if req.ExpectedVersion <= 0 {
		response.BadRequest(c, "expected_version is required")
		return
	}
	req.ID = id
	req.Version = req.ExpectedVersion
	updated, err := svc.UpdateCampaign(c.Request.Context(), req.ReferralCampaign, req.Tiers, getAdminIDFromContext(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *AffiliateHandler) ReferralCampaignParticipants(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := svc.Participants(c.Request.Context(), id, page, pageSize, c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AffiliateHandler) ReferralCampaignInvites(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := svc.Invites(c.Request.Context(), id, page, pageSize, c.Query("search"), c.Query("status"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AffiliateHandler) ReferralCampaignRewards(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := svc.Rewards(c.Request.Context(), id, page, pageSize, c.Query("search"), c.Query("status"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type referralRewardDebtResolutionRequest struct {
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
	Decision        string `json:"decision" binding:"required"`
	Note            string `json:"note" binding:"required"`
}

func (h *AffiliateHandler) ResolveReferralRewardDebt(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	campaignID, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || campaignID <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	rewardID, err := strconv.ParseInt(c.Param("reward_id"), 10, 64)
	if err != nil || rewardID <= 0 {
		response.BadRequest(c, "invalid reward_id")
		return
	}
	var req referralRewardDebtResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	reward, err := svc.ResolveRewardDebt(c.Request.Context(), campaignID, rewardID, req.ExpectedVersion, req.Decision, getAdminIDFromContext(c), req.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, reward)
}

type referralCampaignStatusRequest struct {
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
	Status          string `json:"status" binding:"required"`
	Note            string `json:"note"`
}

func (h *AffiliateHandler) SetReferralCampaignStatus(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	var req referralCampaignStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	campaign, err := svc.SetStatus(c.Request.Context(), id, req.ExpectedVersion, req.Status, getAdminIDFromContext(c), req.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

type referralCampaignReviewRequest struct {
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
	ReviewType      string `json:"review_type" binding:"required"`
	Decision        string `json:"decision" binding:"required"`
	Note            string `json:"note"`
}

func (h *AffiliateHandler) ReviewReferralCampaign(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	var req referralCampaignReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	campaign, err := svc.Review(c.Request.Context(), id, req.ExpectedVersion, req.ReviewType, req.Decision, getAdminIDFromContext(c), req.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

func (h *AffiliateHandler) ReferralCampaignStats(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("campaign_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign_id")
		return
	}
	stats, err := svc.Stats(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *AffiliateHandler) InviteGrowthOverview(c *gin.Context) {
	svc, ok := h.referralCampaignService(c)
	if !ok {
		return
	}
	var campaignID *int64
	if raw := strings.TrimSpace(c.Query("campaign_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "invalid campaign_id")
			return
		}
		campaignID = &id
	}
	overview, err := svc.Overview(c.Request.Context(), campaignID, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}
