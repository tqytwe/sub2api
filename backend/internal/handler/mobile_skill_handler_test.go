package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/mobileskill"
	"github.com/Wei-Shaw/sub2api/ent/usermobileskill"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestMobileSkillHandlerListAndDetail(t *testing.T) {
	client, router := newMobileSkillHandlerTestServer(t, 42)
	published := seedMobileSkill(t, client, "copywriter", "中文文案专家", mobileskill.StatusPublished, 2)
	seedMobileSkill(t, client, "draft", "草稿技能", mobileskill.StatusDraft, 1)
	_, err := client.UserMobileSkill.Create().
		SetUserID(42).
		SetSkillID(published.ID).
		SetInstalledVersion(1).
		SetPinned(true).
		Save(context.Background())
	require.NoError(t, err)

	list := performMobileSkillRequest(t, router, http.MethodGet, "/api/v1/mobile/skills")
	require.Equal(t, http.StatusOK, list.Code)
	var listEnvelope struct {
		Data struct {
			Items []mobileSkillResult `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listEnvelope))
	require.Len(t, listEnvelope.Data.Items, 1)
	require.Equal(t, "中文文案专家", listEnvelope.Data.Items[0].NameZh)
	require.Equal(t, "内容创作", listEnvelope.Data.Items[0].Category)
	require.True(t, listEnvelope.Data.Items[0].Installed)
	require.True(t, listEnvelope.Data.Items[0].Pinned)
	require.NotContains(t, list.Body.String(), `"name":`)
	require.NotContains(t, list.Body.String(), "草稿技能")

	detail := performMobileSkillRequest(t, router, http.MethodGet, "/api/v1/mobile/skills/copywriter")
	require.Equal(t, http.StatusOK, detail.Code)
	var detailEnvelope struct {
		Data mobileSkillResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &detailEnvelope))
	require.NotNil(t, detailEnvelope.Data.Version)
	require.Equal(t, 2, detailEnvelope.Data.Version.Version)
	require.Equal(t, "每次使用按实际模型用量计费", detailEnvelope.Data.Version.ConsumptionNoteZh)
	require.Equal(t, "增加结构化输入", detailEnvelope.Data.Version.ChangelogZh)
	require.Equal(t, "object", detailEnvelope.Data.Version.InputSchema["type"])
	require.NotContains(t, detail.Body.String(), "system_prompt_override")

	notFound := performMobileSkillRequest(t, router, http.MethodGet, "/api/v1/mobile/skills/draft")
	require.Equal(t, http.StatusNotFound, notFound.Code)
}

func TestMobileSkillHandlerInstallAndUninstall(t *testing.T) {
	client, router := newMobileSkillHandlerTestServer(t, 77)
	skill := seedMobileSkill(t, client, "product-photo", "商品摄影助手", mobileskill.StatusPublished, 3)

	install := performMobileSkillRequest(t, router, http.MethodPost, "/api/v1/mobile/skills/product-photo/install")
	require.Equal(t, http.StatusOK, install.Code)
	stored, err := client.UserMobileSkill.Query().
		Where(usermobileskill.UserIDEQ(77), usermobileskill.SkillIDEQ(skill.ID)).
		Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, stored.InstalledVersion)

	_, err = stored.Update().SetInstalledVersion(1).SetPinned(true).Save(context.Background())
	require.NoError(t, err)
	reinstall := performMobileSkillRequest(t, router, http.MethodPost, "/api/v1/mobile/skills/product-photo/install")
	require.Equal(t, http.StatusOK, reinstall.Code)
	stored, err = client.UserMobileSkill.Query().
		Where(usermobileskill.UserIDEQ(77), usermobileskill.SkillIDEQ(skill.ID)).
		Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, stored.InstalledVersion)
	require.True(t, stored.Pinned)

	otherUserInstalled, err := client.UserMobileSkill.Create().
		SetUserID(88).
		SetSkillID(skill.ID).
		SetInstalledVersion(3).
		Save(context.Background())
	require.NoError(t, err)

	uninstall := performMobileSkillRequest(t, router, http.MethodDelete, "/api/v1/mobile/skills/product-photo/install")
	require.Equal(t, http.StatusOK, uninstall.Code)
	exists, err := client.UserMobileSkill.Query().
		Where(usermobileskill.UserIDEQ(77), usermobileskill.SkillIDEQ(skill.ID)).
		Exist(context.Background())
	require.NoError(t, err)
	require.False(t, exists)
	exists, err = client.UserMobileSkill.Query().Where(usermobileskill.IDEQ(otherUserInstalled.ID)).Exist(context.Background())
	require.NoError(t, err)
	require.True(t, exists)

	idempotent := performMobileSkillRequest(t, router, http.MethodDelete, "/api/v1/mobile/skills/product-photo/install")
	require.Equal(t, http.StatusOK, idempotent.Code)
}

func TestMobileSkillHandlerRequiresAuthentication(t *testing.T) {
	db, err := sql.Open("sqlite", "file:mobile_skills_auth?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/skills", NewMobileSkillHandler(client).List)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/skills", nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "请先登录")
}

func newMobileSkillHandlerTestServer(t *testing.T, userID int64) (*dbent.Client, *gin.Engine) {
	t.Helper()
	dsn := fmt.Sprintf("file:mobile_skills_%d?mode=memory&cache=shared&_fk=1", userID)
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	h := NewMobileSkillHandler(client)
	router.GET("/api/v1/mobile/skills", h.List)
	router.GET("/api/v1/mobile/skills/:slug", h.Get)
	router.POST("/api/v1/mobile/skills/:slug/install", h.Install)
	router.DELETE("/api/v1/mobile/skills/:slug/install", h.Uninstall)
	return client, router
}

func seedMobileSkill(t *testing.T, client *dbent.Client, slug, name string, status mobileskill.Status, publishedVersion int) *dbent.MobileSkill {
	t.Helper()
	create := client.MobileSkill.Create().
		SetSlug(slug).
		SetStatus(status).
		SetNameZh(name).
		SetDescriptionZh("面向中文用户的完整技能说明").
		SetCategory("内容创作").
		SetCurrentVersion(publishedVersion)
	if status == mobileskill.StatusPublished {
		create.SetPublishedVersion(publishedVersion)
	}
	skill, err := create.Save(context.Background())
	require.NoError(t, err)
	_, err = client.MobileSkillVersion.Create().
		SetSkillID(skill.ID).
		SetVersion(publishedVersion).
		SetPromptID(1000 + skill.ID).
		SetPromptVersion(4).
		SetInputSchema(map[string]any{"type": "object"}).
		SetExamples([]map[string]any{{"标题": "新品文案"}}).
		SetToolConfig(map[string]any{"需要工具": false}).
		SetModelPolicy(map[string]any{"自动切换模型": false}).
		SetConsumptionNoteZh("每次使用按实际模型用量计费").
		SetChangelogZh("增加结构化输入").
		Save(context.Background())
	require.NoError(t, err)
	return skill
}

func performMobileSkillRequest(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}
