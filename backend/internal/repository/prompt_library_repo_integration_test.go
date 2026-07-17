//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func createPromptLibraryTestUser(t *testing.T) int64 {
	t.Helper()
	var id int64
	email := fmt.Sprintf("prompt-library-%d@example.com", time.Now().UnixNano())
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		INSERT INTO users (email, password_hash, role, status)
		VALUES ($1, 'test', 'user', 'active')
		RETURNING id`, email).Scan(&id))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func cleanupPromptLibraryTables(t *testing.T) {
	t.Helper()
	_, err := integrationDB.ExecContext(context.Background(), `
		DELETE FROM prompt_reports;
		DELETE FROM prompt_review_records;
		DELETE FROM prompt_use_events;
		DELETE FROM prompt_favorites;
		DELETE FROM prompt_import_items;
		DELETE FROM prompt_import_jobs;
		DELETE FROM prompt_media;
		DELETE FROM prompt_category_links;
		DELETE FROM prompt_sources;
		DELETE FROM prompt_versions;
		DELETE FROM prompts;
		DELETE FROM prompt_categories;
	`)
	require.NoError(t, err)
}

func TestPromptLibraryRepositoryPublishesSelectedVersion(t *testing.T) {
	cleanupPromptLibraryTables(t)
	userID := createPromptLibraryTestUser(t)
	repo := NewPromptLibraryRepository(integrationDB)
	ctx := context.Background()

	created, err := repo.SavePrompt(ctx, &service.Prompt{
		BrandType:              service.PromptBrandAuthorized,
		ProvenanceType:         service.PromptProvenanceExternal,
		AuthorizationStatus:    service.PromptAuthorizationAuthorized,
		SourceEvidenceVerified: true,
		TitleZH:                "首版标题",
		DescriptionZH:          "首版说明",
		PromptText:             "first prompt",
		Models:                 []string{"gpt-image-1"},
		Sizes:                  []string{"1024x1024"},
		Sources: []service.PromptSource{{
			SourceKey:           "licensed",
			ExternalID:          "pub-1",
			Evidence:            map[string]any{"license": "approved"},
			AuthorizationStatus: service.PromptAuthorizationAuthorized,
			EvidenceVerified:    true,
		}},
	}, userID)
	require.NoError(t, err)
	require.Equal(t, 1, created.CurrentVersion)

	created.TitleZH = "第二版标题"
	created.DescriptionZH = "第二版说明"
	created.PromptText = "second prompt"
	updated, err := repo.SavePrompt(ctx, created, userID)
	require.NoError(t, err)
	require.Equal(t, 2, updated.CurrentVersion)

	require.NoError(t, repo.AddPromptReview(ctx, service.PromptReviewRecord{
		PromptID:   updated.ID,
		Version:    2,
		Decision:   service.PromptReviewApprove,
		ReviewerID: userID,
	}))
	published, err := repo.SetPromptStatus(ctx, updated.ID, 2, service.PromptStatusPublished, userID)
	require.NoError(t, err)
	require.Equal(t, 2, published.PublishedVersion)

	public, err := repo.GetPrompt(ctx, updated.ID, nil, true)
	require.NoError(t, err)
	require.Equal(t, "second prompt", public.PromptText)
	require.Equal(t, service.PromptStatusPublished, public.Status)
}

func TestPromptLibraryRepositoryKeepsPublishedVersionVisibleDuringReviewAndRollback(t *testing.T) {
	cleanupPromptLibraryTables(t)
	userID := createPromptLibraryTestUser(t)
	repo := NewPromptLibraryRepository(integrationDB)
	ctx := context.Background()

	created, err := repo.SavePrompt(ctx, &service.Prompt{
		BrandType:              service.PromptBrandAuthorized,
		ProvenanceType:         service.PromptProvenanceExternal,
		AuthorizationStatus:    service.PromptAuthorizationAuthorized,
		SourceEvidenceVerified: true,
		TitleZH:                "公开版本",
		DescriptionZH:          "公开版本说明",
		PromptText:             "published prompt",
		Models:                 []string{"gpt-image-1"},
		ReferenceRequirement:   service.PromptReferenceOptional,
		ReferenceInstructions:  "可上传商品参考图以保持包装细节",
		Sources: []service.PromptSource{{
			SourceKey:           "licensed",
			ExternalID:          "review-flow-1",
			Evidence:            map[string]any{"license": "approved"},
			AuthorizationStatus: service.PromptAuthorizationAuthorized,
			EvidenceVerified:    true,
		}},
	}, userID)
	require.NoError(t, err)
	published, err := repo.ApprovePromptVersion(ctx, created.ID, 1, userID, "首版审核通过")
	require.NoError(t, err)
	require.Equal(t, service.PromptReferenceOptional, published.ReferenceRequirement)
	require.Equal(t, "可上传商品参考图以保持包装细节", published.ReferenceInstructions)

	published.TitleZH = "待审核版本"
	published.DescriptionZH = "待审核版本说明"
	published.PromptText = "pending prompt"
	updated, err := repo.SavePrompt(ctx, published, userID)
	require.NoError(t, err)
	require.Equal(t, service.PromptStatusPublished, updated.Status)
	require.Equal(t, 2, updated.CurrentVersion)
	require.Equal(t, 1, updated.PublishedVersion)

	_, err = repo.SubmitPromptReview(ctx, updated.ID, userID)
	require.NoError(t, err)
	public, err := repo.GetPrompt(ctx, updated.ID, nil, true)
	require.NoError(t, err)
	require.Equal(t, "published prompt", public.PromptText)

	rolledBack, err := repo.RollbackPrompt(ctx, updated.ID, 1, userID)
	require.NoError(t, err)
	require.Equal(t, service.PromptStatusPendingReview, rolledBack.Status)
	require.Equal(t, 3, rolledBack.CurrentVersion)
	require.Equal(t, 1, rolledBack.PublishedVersion)

	public, err = repo.GetPrompt(ctx, updated.ID, nil, true)
	require.NoError(t, err)
	require.Equal(t, "published prompt", public.PromptText)

	sources, err := repo.ListPromptSources(ctx, updated.ID)
	require.NoError(t, err)
	require.NotEmpty(t, sources)
	require.Equal(t, 3, sources[0].Version)
}

func TestPromptLibraryRepositoryFavoriteIsConcurrentAndIdempotent(t *testing.T) {
	cleanupPromptLibraryTables(t)
	userID := createPromptLibraryTestUser(t)
	repo := NewPromptLibraryRepository(integrationDB)
	ctx := context.Background()

	prompt, err := repo.SavePrompt(ctx, &service.Prompt{
		BrandType:              service.PromptBrandOriginal,
		ProvenanceType:         service.PromptProvenanceInternal,
		AuthorizationStatus:    service.PromptAuthorizationOriginal,
		SourceEvidenceVerified: true,
		TitleZH:                "并发收藏",
		DescriptionZH:          "并发收藏测试",
		PromptText:             "favorite prompt",
		Models:                 []string{"gpt-image-1"},
		Sources: []service.PromptSource{{
			SourceKey:           "internal-proof",
			ExternalID:          "favorite-original-1",
			OriginalAuthor:      "极速蹬内容团队",
			Evidence:            map[string]any{"summary": "运营复核了原创制作记录", "captured_at": "2026-07-17T04:00:00Z", "proof_type": "internal_creation_record"},
			AuthorizationStatus: service.PromptAuthorizationOriginal,
			EvidenceVerified:    true,
		}},
	}, userID)
	require.NoError(t, err)
	_, err = repo.SetPromptStatus(ctx, prompt.ID, 1, service.PromptStatusPublished, userID)
	require.NoError(t, err)

	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.SetFavorite(ctx, prompt.ID, userID, true)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var favoriteRows int
	var favoriteCount int64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM prompt_favorites WHERE prompt_id = $1 AND user_id = $2`,
		prompt.ID, userID,
	).Scan(&favoriteRows))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT favorite_count FROM prompts WHERE id = $1`, prompt.ID,
	).Scan(&favoriteCount))
	require.Equal(t, 1, favoriteRows)
	require.Equal(t, int64(1), favoriteCount)

	favorites, _, err := repo.ListPrompts(ctx, service.PromptListFilter{
		FavoritedOnly: true,
		Pagination:    pagination.PaginationParams{Page: 1, PageSize: 20},
	}, &userID, true)
	require.NoError(t, err)
	require.Len(t, favorites, 1)
	require.Equal(t, prompt.ID, favorites[0].ID)

	report, err := repo.CreateReport(ctx, service.PromptReport{
		PromptID:   prompt.ID,
		ReporterID: &userID,
		Reason:     "版权或来源问题",
		Detail:     "请核验原始出处",
	})
	require.NoError(t, err)
	require.Equal(t, "open", report.Status)
}

func TestPromptLibraryRepositoryImportUsesDualDeduplication(t *testing.T) {
	cleanupPromptLibraryTables(t)
	userID := createPromptLibraryTestUser(t)
	repo := NewPromptLibraryRepository(integrationDB)
	ctx := context.Background()

	base := service.PromptImportJobInput{
		SourceKey: "catalog-a",
		Items: []service.PromptImportItemInput{{
			ExternalID:     "external-1",
			NormalizedHash: "hash-1",
			TitleZH:        "导入标题",
			DescriptionZH:  "导入说明",
			PromptText:     "import prompt",
			Models:         []string{"gpt-image-1"},
		}},
	}
	first, err := repo.CreateImportJob(ctx, base, userID)
	require.NoError(t, err)
	require.Equal(t, 1, first.ItemCount)

	duplicateExternal := base
	duplicateExternal.Items = append([]service.PromptImportItemInput(nil), base.Items...)
	duplicateExternal.Items[0].NormalizedHash = "hash-2"
	second, err := repo.CreateImportJob(ctx, duplicateExternal, userID)
	require.NoError(t, err)
	require.Equal(t, 0, second.ItemCount)

	duplicateHash := base
	duplicateHash.SourceKey = "catalog-b"
	duplicateHash.Items = append([]service.PromptImportItemInput(nil), base.Items...)
	duplicateHash.Items[0].ExternalID = "external-2"
	third, err := repo.CreateImportJob(ctx, duplicateHash, userID)
	require.NoError(t, err)
	require.Equal(t, 0, third.ItemCount)

	var itemCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_import_items`).Scan(&itemCount))
	require.Equal(t, 1, itemCount)
}
