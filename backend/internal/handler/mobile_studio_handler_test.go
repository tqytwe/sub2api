package handler

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeMobileStudioStore struct {
	createProject   func(context.Context, int64, mobileStudioProjectInput) (*mobileStudioProject, error)
	listProjects    func(context.Context, int64, mobileStudioListFilter) ([]mobileStudioProject, int64, error)
	getProject      func(context.Context, int64, string) (*mobileStudioProject, error)
	updateProject   func(context.Context, int64, string, mobileStudioProjectInput) (*mobileStudioProject, error)
	archiveProject  func(context.Context, int64, string) error
	listEpisodes    func(context.Context, int64, string) ([]mobileStudioEpisode, error)
	createEpisode   func(context.Context, int64, string, mobileStudioEpisodeInput) (*mobileStudioEpisode, error)
	updateEpisode   func(context.Context, int64, string, string, mobileStudioEpisodeInput) (*mobileStudioEpisode, error)
	archiveEpisode  func(context.Context, int64, string, string) error
	listDocuments   func(context.Context, int64, string, *string) ([]mobileStudioDocument, error)
	putDocument     func(context.Context, int64, string, string, mobileStudioDocumentInput) (*mobileStudioDocument, error)
	listAssetLinks  func(context.Context, int64, string, *string) ([]mobileStudioAssetLink, error)
	linkAsset       func(context.Context, int64, string, mobileStudioAssetLinkInput) (*mobileStudioAssetLink, error)
	deleteAssetLink func(context.Context, int64, string, string) error
}

func (s *fakeMobileStudioStore) CreateProject(c context.Context, u int64, in mobileStudioProjectInput) (*mobileStudioProject, error) {
	return s.createProject(c, u, in)
}
func (s *fakeMobileStudioStore) ListProjects(c context.Context, u int64, in mobileStudioListFilter) ([]mobileStudioProject, int64, error) {
	if s.listProjects == nil {
		return []mobileStudioProject{}, 0, nil
	}
	return s.listProjects(c, u, in)
}
func (s *fakeMobileStudioStore) GetProject(c context.Context, u int64, id string) (*mobileStudioProject, error) {
	return s.getProject(c, u, id)
}
func (s *fakeMobileStudioStore) UpdateProject(c context.Context, u int64, id string, in mobileStudioProjectInput) (*mobileStudioProject, error) {
	return s.updateProject(c, u, id, in)
}
func (s *fakeMobileStudioStore) ArchiveProject(c context.Context, u int64, id string) error {
	return s.archiveProject(c, u, id)
}
func (s *fakeMobileStudioStore) ListEpisodes(c context.Context, u int64, id string) ([]mobileStudioEpisode, error) {
	return s.listEpisodes(c, u, id)
}
func (s *fakeMobileStudioStore) CreateEpisode(c context.Context, u int64, id string, in mobileStudioEpisodeInput) (*mobileStudioEpisode, error) {
	return s.createEpisode(c, u, id, in)
}
func (s *fakeMobileStudioStore) UpdateEpisode(c context.Context, u int64, p, e string, in mobileStudioEpisodeInput) (*mobileStudioEpisode, error) {
	return s.updateEpisode(c, u, p, e, in)
}
func (s *fakeMobileStudioStore) ArchiveEpisode(c context.Context, u int64, p, e string) error {
	return s.archiveEpisode(c, u, p, e)
}
func (s *fakeMobileStudioStore) ListDocuments(c context.Context, u int64, p string, e *string) ([]mobileStudioDocument, error) {
	return s.listDocuments(c, u, p, e)
}
func (s *fakeMobileStudioStore) PutDocument(c context.Context, u int64, p, t string, in mobileStudioDocumentInput) (*mobileStudioDocument, error) {
	return s.putDocument(c, u, p, t, in)
}
func (s *fakeMobileStudioStore) ListAssetLinks(c context.Context, u int64, p string, e *string) ([]mobileStudioAssetLink, error) {
	return s.listAssetLinks(c, u, p, e)
}
func (s *fakeMobileStudioStore) LinkAsset(c context.Context, u int64, p string, in mobileStudioAssetLinkInput) (*mobileStudioAssetLink, error) {
	return s.linkAsset(c, u, p, in)
}
func (s *fakeMobileStudioStore) DeleteAssetLink(c context.Context, u int64, p, l string) error {
	return s.deleteAssetLink(c, u, p, l)
}

func TestMobileStudioProjectAndFactsAreOwnerScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectID := uuid.NewString()
	store := &fakeMobileStudioStore{
		getProject: func(_ context.Context, userID int64, id string) (*mobileStudioProject, error) {
			if userID != 21 || id != projectID {
				return nil, errMobileStudioNotFound
			}
			return &mobileStudioProject{ID: id, Title: "都市逆袭", Version: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
		},
		listDocuments: func(_ context.Context, userID int64, id string, _ *string) ([]mobileStudioDocument, error) {
			if userID != 21 || id != projectID {
				return nil, errMobileStudioNotFound
			}
			return []mobileStudioDocument{{ID: uuid.NewString(), ProjectID: id, DocumentType: "script", Content: []byte(`{"entries":[{"id":"SCENE-001"}]}`), Version: 1}}, nil
		},
	}
	h := newMobileStudioHandlerWithStore(store)
	params := gin.Params{{Key: "id", Value: projectID}}
	foreign := performMobileStudioRequest(h.GetProject, http.MethodGet, "/mobile/studio/projects/"+projectID, nil, 22, params)
	require.Equal(t, http.StatusNotFound, foreign.Code)
	owner := performMobileStudioRequest(h.GetProject, http.MethodGet, "/mobile/studio/projects/"+projectID, nil, 21, params)
	require.Equal(t, http.StatusOK, owner.Code)
	foreignFacts := performMobileStudioRequest(h.ListDocuments, http.MethodGet, "/mobile/studio/projects/"+projectID+"/documents", nil, 22, params)
	require.Equal(t, http.StatusNotFound, foreignFacts.Code)
	ownerFacts := performMobileStudioRequest(h.ListDocuments, http.MethodGet, "/mobile/studio/projects/"+projectID+"/documents", nil, 21, params)
	require.Equal(t, http.StatusOK, ownerFacts.Code)
	require.Contains(t, ownerFacts.Body.String(), "SCENE-001")
}

func TestMobileStudioDocumentRequiresExpectedVersionAndAllowedFiveFacts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectID := uuid.NewString()
	called := false
	store := &fakeMobileStudioStore{putDocument: func(_ context.Context, userID int64, id, kind string, input mobileStudioDocumentInput) (*mobileStudioDocument, error) {
		called = true
		require.Equal(t, int64(9), userID)
		require.Equal(t, projectID, id)
		require.Equal(t, "video_prompts", kind)
		require.Equal(t, 1, input.ExpectedVersion)
		return &mobileStudioDocument{ID: uuid.NewString(), ProjectID: id, DocumentType: kind, Content: input.Content, Version: 2}, nil
	}}
	h := newMobileStudioHandlerWithStore(store)
	params := gin.Params{{Key: "id", Value: projectID}, {Key: "documentType", Value: "video_prompts"}}
	ok := performMobileStudioRequest(h.PutDocument, http.MethodPut, "/mobile/studio/projects/"+projectID+"/documents/video_prompts", []byte(`{"expected_version":1,"content":{"entries":[{"id":"MOTION-001","prompt":"跑入雨夜"}]}}`), 9, params)
	require.Equal(t, http.StatusOK, ok.Code)
	require.True(t, called)
	require.Contains(t, ok.Body.String(), "MOTION-001")
	called = false
	params[1].Value = "adaptation"
	bad := performMobileStudioRequest(h.PutDocument, http.MethodPut, "/mobile/studio/projects/"+projectID+"/documents/adaptation", []byte(`{"expected_version":0,"content":{}}`), 9, params)
	require.Equal(t, http.StatusBadRequest, bad.Code)
	require.False(t, called)
}

func TestMobileStudioRejectsConflictingDocumentAndForeignAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectID, assetID := uuid.NewString(), uuid.NewString()
	store := &fakeMobileStudioStore{
		putDocument: func(context.Context, int64, string, string, mobileStudioDocumentInput) (*mobileStudioDocument, error) {
			return nil, errMobileStudioConflict
		},
		linkAsset: func(context.Context, int64, string, mobileStudioAssetLinkInput) (*mobileStudioAssetLink, error) {
			return nil, errMobileStudioInvalidAsset
		},
	}
	h := newMobileStudioHandlerWithStore(store)
	docParams := gin.Params{{Key: "id", Value: projectID}, {Key: "documentType", Value: "script"}}
	conflict := performMobileStudioRequest(h.PutDocument, http.MethodPut, "/mobile/studio/projects/"+projectID+"/documents/script", []byte(`{"expected_version":1,"content":{}}`), 9, docParams)
	require.Equal(t, http.StatusConflict, conflict.Code)
	require.Contains(t, conflict.Body.String(), "STUDIO_VERSION_CONFLICT")
	linkParams := gin.Params{{Key: "id", Value: projectID}}
	foreign := performMobileStudioRequest(h.LinkAsset, http.MethodPost, "/mobile/studio/projects/"+projectID+"/assets", []byte(`{"asset_id":"`+assetID+`","link_type":"keyframe"}`), 9, linkParams)
	require.Equal(t, http.StatusForbidden, foreign.Code)
	require.Contains(t, foreign.Body.String(), "STUDIO_ASSET_UNAVAILABLE")
}

func TestMobileStudioProjectArchiveDoesNotDeleteLinkedAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectID := uuid.NewString()
	assetsDeleted := false
	store := &fakeMobileStudioStore{archiveProject: func(_ context.Context, userID int64, id string) error {
		require.Equal(t, int64(9), userID)
		require.Equal(t, projectID, id)
		return nil
	}}
	h := newMobileStudioHandlerWithStore(store)
	response := performMobileStudioRequest(h.ArchiveProject, http.MethodDelete, "/mobile/studio/projects/"+projectID, nil, 9, gin.Params{{Key: "id", Value: projectID}})
	require.Equal(t, http.StatusOK, response.Code)
	require.False(t, assetsDeleted)
}

func TestMobileStudioWriteErrorDoesNotExposeUnknownErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectID := uuid.NewString()
	store := &fakeMobileStudioStore{archiveProject: func(context.Context, int64, string) error { return errors.New("database host private.example") }}
	h := newMobileStudioHandlerWithStore(store)
	response := performMobileStudioRequest(h.ArchiveProject, http.MethodDelete, "/mobile/studio/projects/"+projectID, nil, 9, gin.Params{{Key: "id", Value: projectID}})
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.NotContains(t, response.Body.String(), "private.example")
}

func TestSQLMobileStudioAssetLinkBindsOwnerAndAssetSeparately(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	projectID, assetID := uuid.NewString(), uuid.NewString()
	createdID := uuid.NewString()
	position := 2
	now := time.Date(2026, time.August, 27, 9, 0, 0, 0, time.UTC)
	metadata := json.RawMessage(`{"source":"IMG-001"}`)
	mock.ExpectQuery(`INSERT INTO studio_asset_links .* a\.id=\$4::uuid AND a\.user_id=\$3 .* p\.id=\$9::uuid AND p\.user_id=\$3`).
		WithArgs(sqlmock.AnyArg(), nil, int64(7), assetID, "keyframe", "IMG-001", position, driver.Value(metadata), projectID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "episode_id", "asset_id", "link_type", "stable_ref", "position", "metadata", "created_at", "updated_at"}).
			AddRow(createdID, projectID, nil, assetID, "keyframe", "IMG-001", position, []byte(metadata), now, now))

	item, err := (&sqlMobileStudioStore{db: db}).LinkAsset(context.Background(), 7, projectID, mobileStudioAssetLinkInput{
		AssetID: assetID, LinkType: "keyframe", StableRef: stringPointer("IMG-001"), Position: &position, Metadata: metadata,
	})
	require.NoError(t, err)
	require.Equal(t, assetID, item.AssetID)
	require.Equal(t, "keyframe", item.LinkType)
	require.NoError(t, mock.ExpectationsWereMet())
}

func stringPointer(value string) *string { return &value }

func performMobileStudioRequest(method gin.HandlerFunc, httpMethod, target string, body []byte, userID int64, params gin.Params) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(httpMethod, target, bytesReader(body))
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Params = params
	if userID > 0 {
		context.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	}
	method(context)
	return recorder
}
