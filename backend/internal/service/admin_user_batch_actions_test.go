//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type batchActionUserRepo struct {
	*userRepoStub
	users      map[int64]*User
	updateErr  map[int64]error
	deleteErr  map[int64]error
	updatedIDs []int64
	deletedIDs []int64
}

func (r *batchActionUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	clone := *user
	return &clone, nil
}

func (r *batchActionUserRepo) Update(_ context.Context, user *User) error {
	r.updatedIDs = append(r.updatedIDs, user.ID)
	if err := r.updateErr[user.ID]; err != nil {
		return err
	}
	clone := *user
	r.users[user.ID] = &clone
	return nil
}

func (r *batchActionUserRepo) Delete(_ context.Context, id int64) error {
	r.deletedIDs = append(r.deletedIDs, id)
	if err := r.deleteErr[id]; err != nil {
		return err
	}
	delete(r.users, id)
	return nil
}

type batchActionAPIKeyRepo struct {
	*apiKeyRepoStub
	keysByUser map[int64][]APIKey
	deleteErr  map[int64]error
	deletedIDs []int64
}

func (r *batchActionAPIKeyRepo) ListByUserID(
	_ context.Context,
	userID int64,
	params pagination.PaginationParams,
	_ APIKeyListFilters,
) ([]APIKey, *pagination.PaginationResult, error) {
	keys := append([]APIKey(nil), r.keysByUser[userID]...)
	return keys, &pagination.PaginationResult{
		Total: int64(len(keys)), Page: params.Page, PageSize: params.PageSize, Pages: 1,
	}, nil
}

func (r *batchActionAPIKeyRepo) DeleteWithAudit(_ context.Context, id int64) error {
	r.deletedIDs = append(r.deletedIDs, id)
	return r.deleteErr[id]
}

func newBatchActionService() (*adminServiceImpl, *batchActionUserRepo, *batchActionAPIKeyRepo) {
	users := &batchActionUserRepo{
		userRepoStub: &userRepoStub{},
		users: map[int64]*User{
			1: {ID: 1, Email: "active@example.test", Role: RoleUser, Status: StatusActive},
			2: {ID: 2, Email: "admin@example.test", Role: RoleAdmin, Status: StatusActive},
			3: {ID: 3, Email: "disabled@example.test", Role: RoleUser, Status: StatusDisabled},
			4: {ID: 4, Email: "second@example.test", Role: RoleUser, Status: StatusActive},
		},
		updateErr: map[int64]error{},
		deleteErr: map[int64]error{},
	}
	keys := &batchActionAPIKeyRepo{
		apiKeyRepoStub: &apiKeyRepoStub{},
		keysByUser: map[int64][]APIKey{
			1: {{ID: 11, UserID: 1, Key: "sk-one"}, {ID: 12, UserID: 1, Key: "sk-two"}},
			3: {{ID: 31, UserID: 3, Key: "sk-three"}},
			4: {{ID: 41, UserID: 4, Key: "sk-four"}},
		},
		deleteErr: map[int64]error{},
	}
	return &adminServiceImpl{
		userRepo:   users,
		apiKeyRepo: keys,
		userBatchPreviewKey: deriveUserBatchPreviewSigningKey(&config.Config{
			JWT: config.JWTConfig{Secret: strings.Repeat("t", 32)},
		}),
	}, users, keys
}

func TestAdminServicePreviewUserBatchActionClassifiesImpact(t *testing.T) {
	svc, _, _ := newBatchActionService()

	preview, err := svc.PreviewUserBatchAction(context.Background(), UserBatchActionInput{
		Action:  UserBatchActionDelete,
		UserIDs: []int64{1, 2, 3, 4, 404, 1},
		Reason:  "confirmed abuse",
	})

	require.NoError(t, err)
	require.Equal(t, 5, preview.RequestedCount)
	require.Equal(t, []int64{1, 3, 4}, batchTargetIDs(preview.EligibleUsers))
	require.Equal(t, []int64{2}, batchTargetIDs(preview.ProtectedAdministrators))
	require.Equal(t, []int64{404}, preview.MissingUserIDs)
	require.Empty(t, preview.AlreadyDisabledUsers)
	require.Equal(t, 4, preview.AffectedAPIKeys)
	require.NotEmpty(t, preview.ConfirmationToken)
	require.True(t, preview.RequiresStepUp)
}

func TestAdminServicePreviewDisableSkipsAlreadyDisabled(t *testing.T) {
	svc, _, _ := newBatchActionService()

	preview, err := svc.PreviewUserBatchAction(context.Background(), UserBatchActionInput{
		Action: UserBatchActionDisable, UserIDs: []int64{1, 2, 3}, Reason: "incident response",
	})

	require.NoError(t, err)
	require.Equal(t, []int64{1}, batchTargetIDs(preview.EligibleUsers))
	require.Equal(t, []int64{2}, batchTargetIDs(preview.ProtectedAdministrators))
	require.Equal(t, []int64{3}, batchTargetIDs(preview.AlreadyDisabledUsers))
	require.Zero(t, preview.AffectedAPIKeys)
}

func TestAdminServiceExecuteDisableUsesPreviewAndContinuesAfterFailure(t *testing.T) {
	svc, users, _ := newBatchActionService()
	users.updateErr[4] = errors.New("write failed")
	input := UserBatchActionInput{
		Action: UserBatchActionDisable, UserIDs: []int64{1, 2, 3, 4}, Reason: "incident response",
		ActorAdminID: 99,
	}
	preview, err := svc.PreviewUserBatchAction(context.Background(), input)
	require.NoError(t, err)
	input.ConfirmationToken = preview.ConfirmationToken

	result, err := svc.ExecuteUserBatchAction(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, UserBatchActionResultPartial, result.Status)
	require.Equal(t, []int64{1}, result.SucceededUserIDs)
	require.Equal(t, []int64{2, 3}, batchResultItemIDs(result.Skipped))
	require.Equal(t, []int64{4}, batchResultItemIDs(result.Failed))
	require.Equal(t, StatusDisabled, users.users[1].Status)
	require.Equal(t, StatusActive, users.users[4].Status)
}

func TestAdminServiceExecuteDeleteRemovesKeysAndContinuesAfterUserFailure(t *testing.T) {
	svc, users, keys := newBatchActionService()
	users.deleteErr[4] = errors.New("delete failed")
	input := UserBatchActionInput{
		Action: UserBatchActionDelete, UserIDs: []int64{1, 4}, Reason: "confirmed abuse",
		ActorAdminID: 99,
	}
	preview, err := svc.PreviewUserBatchAction(context.Background(), input)
	require.NoError(t, err)
	input.ConfirmationToken = preview.ConfirmationToken

	result, err := svc.ExecuteUserBatchAction(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, UserBatchActionResultPartial, result.Status)
	require.Equal(t, []int64{1}, result.SucceededUserIDs)
	require.Equal(t, []int64{4}, batchResultItemIDs(result.Failed))
	require.Equal(t, 2, result.AffectedAPIKeys)
	require.ElementsMatch(t, []int64{11, 12, 41}, keys.deletedIDs)
	_, userOneExists := users.users[1]
	require.False(t, userOneExists)
	require.Contains(t, users.users, int64(4))
}

func TestAdminServiceExecuteRejectsStalePreview(t *testing.T) {
	svc, users, _ := newBatchActionService()
	input := UserBatchActionInput{
		Action: UserBatchActionDisable, UserIDs: []int64{1}, Reason: "incident response",
	}
	preview, err := svc.PreviewUserBatchAction(context.Background(), input)
	require.NoError(t, err)
	users.users[1].Role = RoleAdmin
	input.ConfirmationToken = preview.ConfirmationToken

	_, err = svc.ExecuteUserBatchAction(context.Background(), input)

	require.ErrorIs(t, err, ErrUserBatchActionPreviewStale)
	require.Empty(t, users.updatedIDs)
}

func TestAdminServiceExecuteRejectsForgedPreviewPayload(t *testing.T) {
	svc, users, _ := newBatchActionService()
	input := UserBatchActionInput{
		Action: UserBatchActionDisable, UserIDs: []int64{1}, Reason: "incident response",
	}
	preview, err := svc.PreviewUserBatchAction(context.Background(), input)
	require.NoError(t, err)

	parts := strings.Split(preview.ConfirmationToken, ".")
	require.Len(t, parts, 2)
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var payload userBatchPreviewToken
	require.NoError(t, json.Unmarshal(raw, &payload))
	payload.UserIDs = []int64{4}
	raw, err = json.Marshal(payload)
	require.NoError(t, err)
	input.UserIDs = []int64{4}
	input.ConfirmationToken = base64.RawURLEncoding.EncodeToString(raw) + "." + parts[1]

	_, err = svc.ExecuteUserBatchAction(context.Background(), input)

	require.ErrorIs(t, err, ErrUserBatchActionPreviewInvalid)
	require.Empty(t, users.updatedIDs)
}

func TestAdminServiceExecuteRejectsExpiredOrChangedPreview(t *testing.T) {
	svc, users, _ := newBatchActionService()
	input := UserBatchActionInput{
		Action: UserBatchActionDisable, UserIDs: []int64{1}, Reason: "incident response",
	}
	preview, err := svc.PreviewUserBatchAction(context.Background(), input)
	require.NoError(t, err)

	parts := strings.Split(preview.ConfirmationToken, ".")
	require.Len(t, parts, 2)
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var payload userBatchPreviewToken
	require.NoError(t, json.Unmarshal(raw, &payload))
	payload.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	expiredToken, err := encodeUserBatchPreviewToken(payload, svc.userBatchPreviewKey)
	require.NoError(t, err)
	input.ConfirmationToken = expiredToken

	_, err = svc.ExecuteUserBatchAction(context.Background(), input)
	require.ErrorIs(t, err, ErrUserBatchActionPreviewExpired)

	freshPreview, err := svc.PreviewUserBatchAction(context.Background(), input)
	require.NoError(t, err)
	input.ConfirmationToken = freshPreview.ConfirmationToken
	input.Reason = "different reason"

	_, err = svc.ExecuteUserBatchAction(context.Background(), input)
	require.ErrorIs(t, err, ErrUserBatchActionPreviewStale)
	require.Empty(t, users.updatedIDs)
}

func TestAdminServicePreviewRequiresConfiguredSigner(t *testing.T) {
	svc, _, _ := newBatchActionService()
	svc.userBatchPreviewKey = nil

	_, err := svc.PreviewUserBatchAction(context.Background(), UserBatchActionInput{
		Action: UserBatchActionDisable, UserIDs: []int64{1}, Reason: "incident response",
	})

	require.ErrorIs(t, err, ErrUserBatchActionSignerMissing)
}

func TestAdminServicePreviewUserBatchActionValidatesInput(t *testing.T) {
	svc, _, _ := newBatchActionService()
	tooMany := make([]int64, MaxUserBatchActionUsers+1)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}

	_, err := svc.PreviewUserBatchAction(context.Background(), UserBatchActionInput{
		Action: UserBatchActionDisable, UserIDs: tooMany, Reason: "incident response",
	})
	require.ErrorIs(t, err, ErrUserBatchActionTooLarge)

	_, err = svc.PreviewUserBatchAction(context.Background(), UserBatchActionInput{
		Action: UserBatchActionDisable, UserIDs: []int64{1}, Reason: " ",
	})
	require.ErrorIs(t, err, ErrUserBatchActionReasonRequired)

	_, err = svc.PreviewUserBatchAction(context.Background(), UserBatchActionInput{
		Action: "archive", UserIDs: []int64{1}, Reason: "incident response",
	})
	require.ErrorIs(t, err, ErrUserBatchActionInvalidAction)
}

func batchTargetIDs(items []UserBatchActionTarget) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func batchResultItemIDs(items []UserBatchActionResultItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.UserID)
	}
	return ids
}
