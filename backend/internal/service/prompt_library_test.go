package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type promptLibraryRepoStub struct {
	prompt        *Prompt
	reviews       []PromptReviewRecord
	sources       []PromptSource
	saved         *Prompt
	rolledBackTo  int
	imported      *PromptImportJob
	importedInput PromptImportJobInput
	saveErr       error
}

func (s *promptLibraryRepoStub) GetPrompt(context.Context, int64, *int64, bool) (*Prompt, error) {
	return s.prompt, nil
}

func (s *promptLibraryRepoStub) SavePrompt(_ context.Context, prompt *Prompt, _ int64) (*Prompt, error) {
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	copy := *prompt
	s.saved = &copy
	return &copy, nil
}

func (s *promptLibraryRepoStub) ListPromptSources(context.Context, int64) ([]PromptSource, error) {
	return s.sources, nil
}

func (s *promptLibraryRepoStub) ListPromptReviews(context.Context, int64, int) ([]PromptReviewRecord, error) {
	return s.reviews, nil
}

func (s *promptLibraryRepoStub) SetPromptStatus(_ context.Context, id int64, version int, status PromptStatus, _ int64) (*Prompt, error) {
	out := *s.prompt
	out.ID = id
	out.CurrentVersion = version
	out.Status = status
	s.prompt = &out
	return &out, nil
}

func (s *promptLibraryRepoStub) RollbackPrompt(_ context.Context, _ int64, version int, _ int64) (*Prompt, error) {
	s.rolledBackTo = version
	out := *s.prompt
	out.CurrentVersion = version + 1
	out.Status = PromptStatusPendingReview
	return &out, nil
}

func (s *promptLibraryRepoStub) CreateImportJob(_ context.Context, input PromptImportJobInput, _ int64) (*PromptImportJob, error) {
	s.importedInput = input
	s.imported = &PromptImportJob{SourceKey: input.SourceKey, Status: PromptImportStatusPendingReview}
	return s.imported, nil
}

func promptAuthorizationForBrand(brand PromptBrand) PromptAuthorization {
	switch brand {
	case PromptBrandOriginal:
		return PromptAuthorizationOriginal
	case PromptBrandAuthorized:
		return PromptAuthorizationAuthorized
	case PromptBrandCommunity:
		return PromptAuthorizationCommunity
	default:
		return PromptAuthorizationCurated
	}
}

func validPromptSource(brand PromptBrand, version int) PromptSource {
	evidence := map[string]any{
		"summary":     "运营复核了来源与授权记录",
		"captured_at": "2026-07-17T03:00:00Z",
	}
	if brand == PromptBrandOriginal {
		evidence["proof_type"] = "internal_creation_record"
	}
	return PromptSource{
		Version:             version,
		SourceKey:           "operator-reviewed-source",
		ExternalID:          "source-1",
		OriginalAuthor:      "极速蹬内容团队",
		Evidence:            evidence,
		AuthorizationStatus: promptAuthorizationForBrand(brand),
		EvidenceVerified:    true,
	}
}

func publishablePrompt(id int64, brand PromptBrand, version int) *Prompt {
	return &Prompt{
		ID:                     id,
		Status:                 PromptStatusPendingReview,
		BrandType:              brand,
		CurrentVersion:         version,
		TitleZH:                "商品摄影",
		DescriptionZH:          "适合商品主图",
		PromptText:             "拍摄 {{subject}}",
		Models:                 []string{"gpt-image-1"},
		AuthorizationStatus:    promptAuthorizationForBrand(brand),
		SourceEvidenceVerified: true,
	}
}

func TestPromptServiceDTOIncludesOptimisticLockAndReportDisplayFields(t *testing.T) {
	payload, err := json.Marshal(struct {
		Prompt Prompt       `json:"prompt"`
		Report PromptReport `json:"report"`
	}{
		Prompt: Prompt{ExpectedVersion: 7},
		Report: PromptReport{
			PromptTitle:  "商品摄影",
			ReporterName: "测试用户",
		},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"prompt": {"id":0,"status":"","brand_type":"","provenance_type":"","authorization_status":"","source_evidence_verified":false,"title_zh":"","description_zh":"","purpose":"","style":"","subject":"","featured":false,"current_version":0,"published_version":0,"prompt_text":"","variables":null,"models":null,"sizes":null,"reference_requirement":"","requires_reference":false,"use_count":0,"favorite_count":0,"favorited":false,"category_ids":null,"media":null,"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","expected_version":7},
		"report": {"id":0,"prompt_id":0,"reason":"","detail":"","status":"","resolution":"","created_at":"0001-01-01T00:00:00Z","prompt_title":"商品摄影","reporter_name":"测试用户"}
	}`, string(payload))
}

func TestPromptBrandLabelIsFixed(t *testing.T) {
	require.Equal(t, "极速蹬原创", PromptBrandOriginal.Label())
	require.Equal(t, "极速蹬授权", PromptBrandAuthorized.Label())
	require.Equal(t, "极速蹬精选", PromptBrandCurated.Label())
	require.Equal(t, "极速蹬社区精选", PromptBrandCommunity.Label())
}

func TestValidatePromptProvenanceRejectsUnprovenOriginal(t *testing.T) {
	err := ValidatePromptProvenance(Prompt{
		BrandType:              PromptBrandOriginal,
		ProvenanceType:         PromptProvenanceExternal,
		AuthorizationStatus:    PromptAuthorizationUnknown,
		SourceEvidenceVerified: false,
	})
	require.ErrorIs(t, err, ErrPromptOriginalEvidenceRequired)
}

func TestValidatePromptProvenanceAllowsVerifiedOriginal(t *testing.T) {
	err := ValidatePromptProvenance(Prompt{
		BrandType:              PromptBrandOriginal,
		ProvenanceType:         PromptProvenanceExternal,
		AuthorizationStatus:    PromptAuthorizationOriginal,
		SourceEvidenceVerified: true,
	})
	require.NoError(t, err)
}

func TestValidatePromptProvenanceRejectsAuthorizedContentAsOriginal(t *testing.T) {
	err := ValidatePromptProvenance(Prompt{
		BrandType:              PromptBrandOriginal,
		ProvenanceType:         PromptProvenanceExternal,
		AuthorizationStatus:    PromptAuthorizationAuthorized,
		SourceEvidenceVerified: true,
	})
	require.ErrorIs(t, err, ErrPromptOriginalEvidenceRequired)
}

func TestNormalizeImportedBrandFallsBackToCurated(t *testing.T) {
	require.Equal(t, PromptBrandCurated, NormalizeImportedBrand(
		PromptBrandOriginal,
		PromptAuthorizationUnknown,
		false,
	))
	require.Equal(t, PromptBrandAuthorized, NormalizeImportedBrand(
		PromptBrandAuthorized,
		PromptAuthorizationAuthorized,
		true,
	))
}

func TestPublishRequiresApprovedReviewAndCompleteChineseContent(t *testing.T) {
	repo := &promptLibraryRepoStub{
		prompt:  publishablePrompt(7, PromptBrandAuthorized, 2),
		sources: []PromptSource{validPromptSource(PromptBrandAuthorized, 2)},
		reviews: []PromptReviewRecord{{
			PromptID: 7,
			Version:  2,
			Decision: PromptReviewApprove,
		}},
	}
	svc := NewPromptLibraryService(repo)

	got, err := svc.ApproveAndPublish(context.Background(), 7, 99)
	require.NoError(t, err)
	require.Equal(t, PromptStatusPublished, got.Status)

	repo.reviews = nil
	_, err = svc.ApproveAndPublish(context.Background(), 7, 99)
	require.ErrorIs(t, err, ErrPromptApprovalRequired)

	repo.reviews = []PromptReviewRecord{{PromptID: 7, Version: 2, Decision: PromptReviewApprove}}
	repo.prompt.TitleZH = "english only"
	_, err = svc.ApproveAndPublish(context.Background(), 7, 99)
	require.ErrorIs(t, err, ErrPromptChineseContentRequired)
}

func TestPublishRejectsFakeEvidenceForEveryBrand(t *testing.T) {
	for _, brand := range []PromptBrand{
		PromptBrandOriginal,
		PromptBrandAuthorized,
		PromptBrandCurated,
		PromptBrandCommunity,
	} {
		t.Run(string(brand), func(t *testing.T) {
			source := validPromptSource(brand, 3)
			source.Evidence = map[string]any{"ok": true}
			repo := &promptLibraryRepoStub{
				prompt:  publishablePrompt(11, brand, 3),
				sources: []PromptSource{source},
				reviews: []PromptReviewRecord{{
					PromptID: 11,
					Version:  3,
					Decision: PromptReviewApprove,
				}},
			}

			_, err := NewPromptLibraryService(repo).ApproveAndPublish(context.Background(), 11, 99)
			require.Error(t, err)
			require.Equal(t, "PROMPT_SOURCE_EVIDENCE_REQUIRED", apperrors.Reason(err))
		})
	}
}

func TestPublishRejectsIncompleteOriginalEvidence(t *testing.T) {
	tests := map[string]func(*PromptSource){
		"stale version":      func(source *PromptSource) { source.Version-- },
		"missing source key": func(source *PromptSource) { source.SourceKey = "" },
		"missing external id": func(source *PromptSource) {
			source.ExternalID = ""
		},
		"missing original author": func(source *PromptSource) {
			source.OriginalAuthor = ""
		},
		"missing summary": func(source *PromptSource) { delete(source.Evidence, "summary") },
		"invalid captured at": func(source *PromptSource) {
			source.Evidence["captured_at"] = "yesterday"
		},
		"missing proof type": func(source *PromptSource) { delete(source.Evidence, "proof_type") },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			source := validPromptSource(PromptBrandOriginal, 4)
			mutate(&source)
			repo := &promptLibraryRepoStub{
				prompt:  publishablePrompt(12, PromptBrandOriginal, 4),
				sources: []PromptSource{source},
				reviews: []PromptReviewRecord{{
					PromptID: 12,
					Version:  4,
					Decision: PromptReviewApprove,
				}},
			}

			_, err := NewPromptLibraryService(repo).ApproveAndPublish(context.Background(), 12, 99)
			require.Error(t, err)
			require.Equal(t, "PROMPT_SOURCE_EVIDENCE_REQUIRED", apperrors.Reason(err))
		})
	}
}

func TestPublishRejectsUnverifiedSourceEvidence(t *testing.T) {
	repo := &promptLibraryRepoStub{
		prompt: publishablePrompt(9, PromptBrandAuthorized, 1),
		reviews: []PromptReviewRecord{{
			PromptID: 9,
			Version:  1,
			Decision: PromptReviewApprove,
		}},
	}
	source := validPromptSource(PromptBrandAuthorized, 1)
	source.EvidenceVerified = false
	repo.sources = []PromptSource{source}
	svc := NewPromptLibraryService(repo)

	_, err := svc.ApproveAndPublish(context.Background(), 9, 3)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source evidence")
}

func TestImportJobAlwaysStartsPendingReview(t *testing.T) {
	repo := &promptLibraryRepoStub{}
	svc := NewPromptLibraryService(repo)

	job, err := svc.CreateImportJob(context.Background(), PromptImportJobInput{
		SourceKey: "operator-json",
		Status:    PromptImportStatus("published"),
		Items: []PromptImportItemInput{{
			ExternalID:     "ext-1",
			NormalizedHash: "hash-1",
			TitleZH:        "测试",
			PromptText:     "正文",
		}},
	}, 3)
	require.NoError(t, err)
	require.Equal(t, PromptImportStatusPendingReview, job.Status)
}

func TestImportJobBuildsStableDeduplicationKeys(t *testing.T) {
	repo := &promptLibraryRepoStub{}
	svc := NewPromptLibraryService(repo)
	input := PromptImportJobInput{
		SourceKey: "operator-json",
		Items: []PromptImportItemInput{{
			NormalizedHash: "caller-controlled-hash",
			TitleZH:        "测试标题",
			PromptText:     "Create a clean product image",
			Models:         []string{"gpt-image-1"},
		}},
	}

	first, err := svc.CreateImportJob(context.Background(), input, 3)
	require.NoError(t, err)
	require.Equal(t, PromptImportStatusPendingReview, first.Status)
	require.NotEmpty(t, repo.importedInput.Items[0].NormalizedHash)
	require.NotEqual(t, "caller-controlled-hash", repo.importedInput.Items[0].NormalizedHash)
	require.Equal(t, repo.importedInput.Items[0].NormalizedHash, repo.importedInput.Items[0].ExternalID)
}

func TestImportJobRejectsSecretMaterial(t *testing.T) {
	repo := &promptLibraryRepoStub{}
	svc := NewPromptLibraryService(repo)

	_, err := svc.CreateImportJob(context.Background(), PromptImportJobInput{
		SourceKey: "operator-json",
		Items: []PromptImportItemInput{{
			TitleZH:    "测试标题",
			PromptText: "prompt",
			Evidence: map[string]any{
				"access_token": "secret",
			},
		}},
	}, 3)
	require.Error(t, err)
	require.Contains(t, err.Error(), "PROMPT_IMPORT_SECRET_REJECTED")
}

func TestImportJobScansEntireItemForSecretKeysAndValues(t *testing.T) {
	tests := map[string]func(*PromptImportItemInput){
		"accessToken key": func(item *PromptImportItemInput) {
			item.Variables = map[string]any{"accessToken": "redacted"}
		},
		"authorization key": func(item *PromptImportItemInput) {
			item.Variables = map[string]any{"authorization": "redacted"}
		},
		"x-api-key key": func(item *PromptImportItemInput) {
			item.Variables = map[string]any{"x-api-key": "redacted"}
		},
		"OpenAI key value": func(item *PromptImportItemInput) {
			item.DescriptionZH = "sk-proj-1234567890abcdefghijklmnop"
		},
		"GitHub token value": func(item *PromptImportItemInput) {
			item.PromptText = "ghp_1234567890abcdefghijklmnopqrstuv"
		},
		"JWT value": func(item *PromptImportItemInput) {
			item.ReferenceInstructions = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.c2lnbmF0dXJl"
		},
		"private key value": func(item *PromptImportItemInput) {
			item.Subject = "-----BEGIN RSA PRIVATE KEY-----\nredacted\n-----END RSA PRIVATE KEY-----"
		},
		"Bearer value": func(item *PromptImportItemInput) {
			item.SourceURL = "Bearer opaque-access-token"
		},
		"Cookie value": func(item *PromptImportItemInput) {
			item.Style = "Cookie: session_id=abc123; csrftoken=def456"
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			item := PromptImportItemInput{
				TitleZH:    "测试标题",
				PromptText: "普通提示词",
			}
			mutate(&item)
			_, err := NewPromptLibraryService(&promptLibraryRepoStub{}).CreateImportJob(
				context.Background(),
				PromptImportJobInput{
					SourceKey: "operator-json",
					Items:     []PromptImportItemInput{item},
				},
				3,
			)
			require.Error(t, err)
			require.Equal(t, "PROMPT_IMPORT_SECRET_REJECTED", apperrors.Reason(err))
		})
	}
}

func TestSavePromptRequiresStrictCurrentVersionEvidence(t *testing.T) {
	tests := map[string]func(*PromptSource){
		"stale version":      func(source *PromptSource) { source.Version-- },
		"missing source key": func(source *PromptSource) { source.SourceKey = "" },
		"missing external id": func(source *PromptSource) {
			source.ExternalID = ""
		},
		"missing original author": func(source *PromptSource) {
			source.OriginalAuthor = ""
		},
		"missing summary": func(source *PromptSource) { delete(source.Evidence, "summary") },
		"missing captured at": func(source *PromptSource) {
			delete(source.Evidence, "captured_at")
		},
		"missing proof type": func(source *PromptSource) { delete(source.Evidence, "proof_type") },
		"unverified":         func(source *PromptSource) { source.EvidenceVerified = false },
		"wrong authorization": func(source *PromptSource) {
			source.AuthorizationStatus = PromptAuthorizationAuthorized
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			source := validPromptSource(PromptBrandOriginal, 5)
			mutate(&source)
			prompt := publishablePrompt(20, PromptBrandOriginal, 5)
			prompt.Sources = []PromptSource{source}

			_, err := NewPromptLibraryService(&promptLibraryRepoStub{}).SavePrompt(
				context.Background(),
				prompt,
				99,
			)
			require.Error(t, err)
			require.Equal(t, "PROMPT_SOURCE_EVIDENCE_REQUIRED", apperrors.Reason(err))
		})
	}
}

func TestSavePromptRejectsFakeEvidenceForEveryBrand(t *testing.T) {
	for _, brand := range []PromptBrand{
		PromptBrandOriginal,
		PromptBrandAuthorized,
		PromptBrandCurated,
		PromptBrandCommunity,
	} {
		t.Run(string(brand), func(t *testing.T) {
			source := validPromptSource(brand, 2)
			source.Evidence = map[string]any{"ok": true}
			prompt := publishablePrompt(21, brand, 2)
			prompt.Sources = []PromptSource{source}

			_, err := NewPromptLibraryService(&promptLibraryRepoStub{}).SavePrompt(
				context.Background(),
				prompt,
				99,
			)
			require.Error(t, err)
			require.Equal(t, "PROMPT_SOURCE_EVIDENCE_REQUIRED", apperrors.Reason(err))
		})
	}
}

func TestSavePromptMapsVersionConflictToHTTP409(t *testing.T) {
	repo := &promptLibraryRepoStub{saveErr: ErrPromptVersionConflict}
	prompt := publishablePrompt(22, PromptBrandAuthorized, 6)
	prompt.ExpectedVersion = 6
	prompt.Sources = []PromptSource{validPromptSource(PromptBrandAuthorized, 6)}

	_, err := NewPromptLibraryService(repo).SavePrompt(context.Background(), prompt, 99)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrPromptVersionConflict))
	require.Equal(t, http.StatusConflict, apperrors.Code(err))
	require.Equal(t, "PROMPT_VERSION_CONFLICT", apperrors.Reason(err))
}

func TestRollbackRejectsCurrentOrFutureVersion(t *testing.T) {
	repo := &promptLibraryRepoStub{prompt: &Prompt{
		ID:               8,
		Status:           PromptStatusPublished,
		CurrentVersion:   4,
		PublishedVersion: 4,
	}}
	svc := NewPromptLibraryService(repo)

	_, err := svc.RollbackVersion(context.Background(), 8, 4, 9)
	require.ErrorIs(t, err, ErrPromptRollbackVersionInvalid)

	got, err := svc.RollbackVersion(context.Background(), 8, 2, 9)
	require.NoError(t, err)
	require.Equal(t, 2, repo.rolledBackTo)
	require.Equal(t, PromptStatusPendingReview, got.Status)
	require.Equal(t, 4, got.PublishedVersion)
}
