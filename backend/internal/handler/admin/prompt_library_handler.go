package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PromptLibraryHandler struct {
	service *service.PromptLibraryService
}

func NewPromptLibraryHandler(promptService *service.PromptLibraryService) *PromptLibraryHandler {
	return &PromptLibraryHandler{service: promptService}
}

func (h *PromptLibraryHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	rows, result, err := h.service.ListAdmin(c.Request.Context(), service.PromptListFilter{
		Query:      c.Query("q"),
		Purpose:    c.Query("purpose"),
		Style:      c.Query("style"),
		Subject:    c.Query("subject"),
		Model:      c.Query("model"),
		Size:       c.Query("size"),
		Status:     service.PromptStatus(c.Query("status")),
		Sort:       c.Query("sort"),
		Pagination: pagination.PaginationParams{Page: page, PageSize: pageSize},
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

func (h *PromptLibraryHandler) Get(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	prompt, err := h.service.GetAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prompt)
}

func (h *PromptLibraryHandler) Create(c *gin.Context) {
	var prompt service.Prompt
	if err := c.ShouldBindJSON(&prompt); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	saved, err := h.service.SavePrompt(c.Request.Context(), &prompt, adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, saved)
}

func (h *PromptLibraryHandler) Update(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	var prompt service.Prompt
	if err := c.ShouldBindJSON(&prompt); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	prompt.ID = id
	saved, err := h.service.SavePrompt(c.Request.Context(), &prompt, adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, saved)
}

func (h *PromptLibraryHandler) SubmitReview(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	prompt, err := h.service.SubmitForReview(c.Request.Context(), id, adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prompt)
}

func (h *PromptLibraryHandler) Approve(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	prompt, err := h.service.ReviewAndPublish(c.Request.Context(), id, adminActorID(c), req.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prompt)
}

func (h *PromptLibraryHandler) Offline(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	prompt, err := h.service.Offline(c.Request.Context(), id, adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prompt)
}

func (h *PromptLibraryHandler) Rollback(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	var req struct {
		Version int `json:"version" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	prompt, err := h.service.RollbackVersion(c.Request.Context(), id, req.Version, adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prompt)
}

func (h *PromptLibraryHandler) ListCategories(c *gin.Context) {
	rows, err := h.service.ListAdminCategories(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rows)
}

func (h *PromptLibraryHandler) CreateCategory(c *gin.Context) {
	var category service.PromptCategory
	if err := c.ShouldBindJSON(&category); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	saved, err := h.service.SaveCategory(c.Request.Context(), &category)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, saved)
}

func (h *PromptLibraryHandler) UpdateCategory(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	var category service.PromptCategory
	if err := c.ShouldBindJSON(&category); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	category.ID = id
	saved, err := h.service.SaveCategory(c.Request.Context(), &category)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, saved)
}

func (h *PromptLibraryHandler) DeleteCategory(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteCategory(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *PromptLibraryHandler) CreateImportJob(c *gin.Context) {
	var input service.PromptImportJobInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	job, err := h.service.CreateImportJob(c.Request.Context(), input, adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, job)
}

func (h *PromptLibraryHandler) ListImportJobs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	rows, result, err := h.service.ListImportJobs(c.Request.Context(), pagination.PaginationParams{
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

func (h *PromptLibraryHandler) GetImportJob(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	job, err := h.service.GetImportJob(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

func (h *PromptLibraryHandler) ListImportItems(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	jobID, _ := strconv.ParseInt(c.Query("job_id"), 10, 64)
	rows, result, err := h.service.ListImportItems(c.Request.Context(), service.PromptImportItemListFilter{
		JobID:      jobID,
		Status:     service.PromptImportItemStatus(c.Query("status")),
		Pagination: pagination.PaginationParams{Page: page, PageSize: pageSize},
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

func (h *PromptLibraryHandler) ApproveImportItem(c *gin.Context) {
	h.reviewImportItem(c, true)
}

func (h *PromptLibraryHandler) RejectImportItem(c *gin.Context) {
	h.reviewImportItem(c, false)
}

func (h *PromptLibraryHandler) reviewImportItem(c *gin.Context, approve bool) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !approve {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	item, err := h.service.ReviewImportItem(c.Request.Context(), id, adminActorID(c), approve, req.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *PromptLibraryHandler) ListReports(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	rows, result, err := h.service.ListReports(c.Request.Context(), service.PromptReportListFilter{
		Status:     c.Query("status"),
		Pagination: pagination.PaginationParams{Page: page, PageSize: pageSize},
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

func (h *PromptLibraryHandler) ResolveReport(c *gin.Context) {
	id, ok := adminPromptPathID(c)
	if !ok {
		return
	}
	var req struct {
		Status     string `json:"status" binding:"required"`
		Resolution string `json:"resolution" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	report, err := h.service.ResolveReport(
		c.Request.Context(), id, adminActorID(c), req.Status, req.Resolution,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}

func adminPromptPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return 0, false
	}
	return id, true
}

func adminActorID(c *gin.Context) int64 {
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	return subject.UserID
}
