package admin

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *CouponHandler) ClaimPolicy(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	r, err := h.service.CampaignRepository()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if c.Request.Method == "PUT" {
		var input service.CouponClaimPolicy
		if err = c.ShouldBindJSON(&input); err != nil {
			response.BadRequest(c, "invalid claim settings")
			return
		}
		p, e := r.SaveCouponClaimPolicy(c.Request.Context(), id, input, adminActorID(c))
		if e != nil {
			response.ErrorFrom(c, e)
			return
		}
		response.Success(c, p)
		return
	}
	p, err := r.GetCouponClaimPolicy(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *CouponHandler) PreviewCampaign(c *gin.Context) {
	var input struct {
		TemplateID int64                  `json:"template_id" binding:"required,gt=0"`
		Audience   service.CouponAudience `json:"audience"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid coupon audience")
		return
	}
	r, err := h.service.CampaignRepository()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.PreviewCouponCampaign(c.Request.Context(), input.TemplateID, input.Audience, adminActorID(c), time.Now().UTC())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *CouponHandler) ExecuteCampaign(c *gin.Context) {
	var input struct {
		Token string `json:"token" binding:"required,len=48"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "preview required")
		return
	}
	r, err := h.service.CampaignRepository()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.ExecuteCouponCampaign(c.Request.Context(), input.Token, adminActorID(c), time.Now().UTC())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *CouponHandler) Summary(c *gin.Context) {
	f := service.UserCouponListFilter{UserID: couponOptionalInt64(c.Query("user_id")), UserQuery: strings.TrimSpace(c.Query("user")), TemplateID: couponOptionalInt64(c.Query("template_id")), Source: service.CouponIssueSource(c.Query("source")), Status: service.UserCouponStatus(c.Query("status")), IssuedFrom: couponOptionalTime(c.Query("issued_from")), IssuedTo: couponOptionalTime(c.Query("issued_to"))}
	p, err := h.service.CampaignSummary(c.Request.Context(), f)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *CouponHandler) Events(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	r, err := h.service.CampaignRepository()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.CouponEvents(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}

func (h *CouponHandler) ClaimPolicies(c *gin.Context) {
	parts := strings.Split(c.Query("ids"), ",")
	if len(parts) > 1000 {
		response.BadRequest(c, "too many templates")
		return
	}
	ids := []int64{}
	for _, part := range parts {
		id := couponOptionalInt64(part)
		if id <= 0 {
			response.BadRequest(c, "invalid template id")
			return
		}
		ids = append(ids, id)
	}
	r, err := h.service.CampaignRepository()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.ListCouponClaimPolicies(c.Request.Context(), ids)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
