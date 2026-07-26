package handler

import (
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/mobileskill"
	"github.com/Wei-Shaw/sub2api/ent/mobileskillversion"
	"github.com/Wei-Shaw/sub2api/ent/usermobileskill"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type MobileSkillHandler struct {
	client *dbent.Client
}

type mobileSkillVersionResult struct {
	Version           int              `json:"version"`
	PromptID          int64            `json:"prompt_id"`
	PromptVersion     int              `json:"prompt_version"`
	InputSchema       map[string]any   `json:"input_schema"`
	Examples          []map[string]any `json:"examples"`
	ToolConfig        map[string]any   `json:"tool_config"`
	ModelPolicy       map[string]any   `json:"model_policy"`
	ConsumptionNoteZh string           `json:"consumption_note_zh"`
	ChangelogZh       string           `json:"changelog_zh"`
	SystemPrompt      string           `json:"system_prompt"`
}

type mobileSkillResult struct {
	ID               int64                     `json:"id"`
	Slug             string                    `json:"slug"`
	NameZh           string                    `json:"name_zh"`
	DescriptionZh    string                    `json:"description_zh"`
	Category         string                    `json:"category"`
	IconURL          string                    `json:"icon_url"`
	CoverURL         string                    `json:"cover_url"`
	Featured         bool                      `json:"featured"`
	PublishedVersion int                       `json:"published_version"`
	Installed        bool                      `json:"installed"`
	InstalledVersion *int                      `json:"installed_version,omitempty"`
	Pinned           bool                      `json:"pinned"`
	LastUsedAt       *time.Time                `json:"last_used_at,omitempty"`
	Version          *mobileSkillVersionResult `json:"version,omitempty"`
}

func NewMobileSkillHandler(client *dbent.Client) *MobileSkillHandler {
	return &MobileSkillHandler{client: client}
}

// NewMobileSkillHandlerFromAuth reuses the Ent client already provided to the
// authentication service, avoiding another wire dependency for this small API.
func NewMobileSkillHandlerFromAuth(auth *AuthHandler) *MobileSkillHandler {
	if auth == nil || auth.authService == nil {
		return NewMobileSkillHandler(nil)
	}
	return NewMobileSkillHandler(auth.authService.EntClient())
}

func (h *MobileSkillHandler) List(c *gin.Context) {
	userID, ok := mobileSkillUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}

	rows, err := h.client.MobileSkill.Query().
		Where(mobileskill.StatusEQ(mobileskill.StatusPublished), mobileskill.PublishedVersionNotNil()).
		WithUserInstalls(func(q *dbent.UserMobileSkillQuery) {
			q.Where(usermobileskill.UserIDEQ(userID))
		}).
		Order(dbent.Asc(mobileskill.FieldSortOrder), dbent.Asc(mobileskill.FieldID)).
		All(c.Request.Context())
	if err != nil {
		response.InternalError(c, "技能目录暂时不可用")
		return
	}

	items := make([]mobileSkillResult, 0, len(rows))
	for _, row := range rows {
		items = append(items, mobileSkillResultFromEntity(row, nil))
	}
	response.Success(c, gin.H{"items": items})
}

func (h *MobileSkillHandler) Get(c *gin.Context) {
	userID, ok := mobileSkillUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}

	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		response.BadRequest(c, "技能标识不能为空")
		return
	}

	row, err := h.client.MobileSkill.Query().
		Where(mobileskill.SlugEQ(slug), mobileskill.StatusEQ(mobileskill.StatusPublished), mobileskill.PublishedVersionNotNil()).
		WithUserInstalls(func(q *dbent.UserMobileSkillQuery) {
			q.Where(usermobileskill.UserIDEQ(userID))
		}).
		Only(c.Request.Context())
	if dbent.IsNotFound(err) {
		response.NotFound(c, "技能不存在或尚未发布")
		return
	}
	if err != nil {
		response.InternalError(c, "技能详情暂时不可用")
		return
	}

	version, err := h.publishedVersion(c, row)
	if dbent.IsNotFound(err) {
		response.NotFound(c, "技能版本尚未发布")
		return
	}
	if err != nil {
		response.InternalError(c, "技能详情暂时不可用")
		return
	}

	response.Success(c, mobileSkillResultFromEntity(row, version))
}

func (h *MobileSkillHandler) Install(c *gin.Context) {
	userID, ok := mobileSkillUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}

	row, err := h.findPublishedSkill(c, strings.TrimSpace(c.Param("slug")))
	if dbent.IsNotFound(err) {
		response.NotFound(c, "技能不存在或尚未发布")
		return
	}
	if err != nil {
		response.InternalError(c, "技能安装失败")
		return
	}

	installedVersion := *row.PublishedVersion
	err = h.client.UserMobileSkill.Create().
		SetUserID(userID).
		SetSkillID(row.ID).
		SetInstalledVersion(installedVersion).
		OnConflictColumns(usermobileskill.FieldUserID, usermobileskill.FieldSkillID).
		SetInstalledVersion(installedVersion).
		SetUpdatedAt(time.Now()).
		Exec(c.Request.Context())
	if err != nil {
		response.InternalError(c, "技能安装失败")
		return
	}
	install, err := h.client.UserMobileSkill.Query().
		Where(usermobileskill.UserIDEQ(userID), usermobileskill.SkillIDEQ(row.ID)).
		Only(c.Request.Context())
	if err != nil {
		response.InternalError(c, "技能安装失败")
		return
	}

	response.Success(c, gin.H{
		"installed":         true,
		"installed_version": install.InstalledVersion,
		"pinned":            install.Pinned,
	})
}

func (h *MobileSkillHandler) Uninstall(c *gin.Context) {
	userID, ok := mobileSkillUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}

	slug := strings.TrimSpace(c.Param("slug"))
	row, err := h.client.MobileSkill.Query().Where(mobileskill.SlugEQ(slug)).Only(c.Request.Context())
	if dbent.IsNotFound(err) {
		response.Success(c, gin.H{"installed": false})
		return
	}
	if err != nil {
		response.InternalError(c, "技能卸载失败")
		return
	}

	_, err = h.client.UserMobileSkill.Delete().
		Where(usermobileskill.UserIDEQ(userID), usermobileskill.SkillIDEQ(row.ID)).
		Exec(c.Request.Context())
	if err != nil {
		response.InternalError(c, "技能卸载失败")
		return
	}
	response.Success(c, gin.H{"installed": false})
}

func (h *MobileSkillHandler) Use(c *gin.Context) {
	userID, ok := mobileSkillUserID(c)
	if !ok || !h.available(c) {
		return
	}
	row, err := h.findPublishedSkill(c, strings.TrimSpace(c.Param("slug")))
	if dbent.IsNotFound(err) {
		response.NotFound(c, "技能不存在或尚未发布")
		return
	}
	if err != nil {
		response.InternalError(c, "技能暂时不可用")
		return
	}
	version, err := h.publishedVersion(c, row)
	if err != nil {
		response.InternalError(c, "技能版本暂时不可用")
		return
	}
	now := time.Now().UTC()
	installedVersion := *row.PublishedVersion
	if err := h.client.UserMobileSkill.Create().
		SetUserID(userID).
		SetSkillID(row.ID).
		SetInstalledVersion(installedVersion).
		SetLastUsedAt(now).
		OnConflictColumns(usermobileskill.FieldUserID, usermobileskill.FieldSkillID).
		SetInstalledVersion(installedVersion).
		SetLastUsedAt(now).
		SetUpdatedAt(now).
		Exec(c.Request.Context()); err != nil {
		response.InternalError(c, "记录技能使用失败")
		return
	}
	response.Success(c, mobileSkillResultFromEntity(row, version))
}

func (h *MobileSkillHandler) findPublishedSkill(c *gin.Context, slug string) (*dbent.MobileSkill, error) {
	return h.client.MobileSkill.Query().
		Where(mobileskill.SlugEQ(slug), mobileskill.StatusEQ(mobileskill.StatusPublished), mobileskill.PublishedVersionNotNil()).
		Only(c.Request.Context())
}

func (h *MobileSkillHandler) publishedVersion(c *gin.Context, skill *dbent.MobileSkill) (*dbent.MobileSkillVersion, error) {
	if skill == nil || skill.PublishedVersion == nil {
		return nil, &dbent.NotFoundError{}
	}
	return h.client.MobileSkillVersion.Query().
		Where(mobileskillversion.SkillIDEQ(skill.ID), mobileskillversion.VersionEQ(*skill.PublishedVersion)).
		Only(c.Request.Context())
}

func (h *MobileSkillHandler) available(c *gin.Context) bool {
	if h != nil && h.client != nil {
		return true
	}
	response.InternalError(c, "技能中心暂时不可用")
	return false
}

func mobileSkillUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return 0, false
	}
	return subject.UserID, true
}

func mobileSkillResultFromEntity(skill *dbent.MobileSkill, version *dbent.MobileSkillVersion) mobileSkillResult {
	result := mobileSkillResult{
		ID:            skill.ID,
		Slug:          skill.Slug,
		NameZh:        skill.NameZh,
		DescriptionZh: skill.DescriptionZh,
		Category:      skill.Category,
		IconURL:       skill.IconURL,
		CoverURL:      skill.CoverURL,
		Featured:      skill.Featured,
	}
	if skill.PublishedVersion != nil {
		result.PublishedVersion = *skill.PublishedVersion
	}
	if len(skill.Edges.UserInstalls) > 0 {
		install := skill.Edges.UserInstalls[0]
		result.Installed = true
		result.InstalledVersion = &install.InstalledVersion
		result.Pinned = install.Pinned
		result.LastUsedAt = install.LastUsedAt
	}
	if version != nil {
		result.Version = &mobileSkillVersionResult{
			Version:           version.Version,
			PromptID:          version.PromptID,
			PromptVersion:     version.PromptVersion,
			InputSchema:       version.InputSchema,
			Examples:          version.Examples,
			ToolConfig:        version.ToolConfig,
			ModelPolicy:       version.ModelPolicy,
			ConsumptionNoteZh: version.ConsumptionNoteZh,
			ChangelogZh:       version.ChangelogZh,
			SystemPrompt:      mobileSkillPrompt(version.SystemPromptOverride),
		}
	}
	return result
}

func mobileSkillPrompt(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
