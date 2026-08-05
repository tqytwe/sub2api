package admin

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	exclusiveGroupPreviewTTL = 10 * time.Minute
	maxExclusiveGroupUsers   = 5000
	maxCSVBytes              = 5 << 20
	maxCSVRows               = 10000
)

var (
	errExclusiveGroupRequired = infraerrors.BadRequest("EXCLUSIVE_STANDARD_GROUP_REQUIRED", "a standard exclusive group is required")
	errPreviewExpired         = infraerrors.BadRequest("PREVIEW_EXPIRED", "the preview has expired")
	errPreviewMismatch        = infraerrors.Conflict("PREVIEW_MISMATCH", "the request no longer matches the preview")
	errCSVInvalid             = infraerrors.BadRequest("CSV_INVALID", "the CSV file is invalid")
)

var exclusivePreviewSecret = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("generate exclusive group preview secret")
	}
	return key
}()

func exclusivePreviewDigest(data []byte) [sha256.Size]byte {
	mac := hmac.New(sha256.New, exclusivePreviewSecret)
	_, _ = mac.Write(data)
	var digest [sha256.Size]byte
	copy(digest[:], mac.Sum(nil))
	return digest
}

type exclusiveGroupBatchRequest struct {
	GroupID int64                    `json:"group_id" binding:"required,gt=0"`
	Action  string                   `json:"action" binding:"required,oneof=grant revoke"`
	UserIDs []int64                  `json:"user_ids"`
	All     bool                     `json:"all"`
	Filters exclusiveGroupUserFilter `json:"filters"`
	Preview string                   `json:"preview_token"`
}

type exclusiveGroupUserFilter struct {
	Status      string           `json:"status,omitempty"`
	Role        string           `json:"role,omitempty"`
	Search      string           `json:"search,omitempty"`
	GroupName   string           `json:"group_name,omitempty"`
	APIKeyGroup int64            `json:"api_key_group_id,omitempty"`
	VIPTier     *int             `json:"vip_tier,omitempty"`
	Attributes  map[int64]string `json:"attributes,omitempty"`
}

type exclusiveGroupTarget struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email,omitempty"`
	Reason string `json:"reason"`
}

type exclusiveGroupBatchPreview struct {
	GroupID        int64                  `json:"group_id"`
	GroupName      string                 `json:"group_name"`
	Action         string                 `json:"action"`
	RequestedCount int                    `json:"requested_count"`
	Eligible       []exclusiveGroupTarget `json:"eligible"`
	AlreadyGranted []exclusiveGroupTarget `json:"already_granted"`
	AlreadyRevoked []exclusiveGroupTarget `json:"already_revoked"`
	Skipped        []exclusiveGroupTarget `json:"skipped"`
	MissingUserIDs []int64                `json:"missing_user_ids"`
	PreviewToken   string                 `json:"preview_token"`
	ExpiresAt      time.Time              `json:"expires_at"`
}

type exclusiveGroupBatchResult struct {
	Action   string                 `json:"action"`
	Affected int                    `json:"affected"`
	Skipped  []exclusiveGroupTarget `json:"skipped"`
}

type csvPreviewResponse struct {
	FileSHA256     string                 `json:"file_sha256"`
	GroupID        int64                  `json:"group_id"`
	GroupName      string                 `json:"group_name"`
	Action         string                 `json:"action"`
	Valid          []exclusiveGroupTarget `json:"valid"`
	AlreadyGranted []exclusiveGroupTarget `json:"already_granted"`
	AlreadyRevoked []exclusiveGroupTarget `json:"already_revoked"`
	DuplicateRows  []int                  `json:"duplicate_rows"`
	NotFound       []string               `json:"not_found"`
	Disabled       []string               `json:"disabled"`
	Invalid        []string               `json:"invalid"`
	PreviewToken   string                 `json:"preview_token"`
	ExpiresAt      time.Time              `json:"expires_at"`
}

func (h *UserHandler) validateExclusiveGroup(c *gin.Context, groupID int64) (*service.Group, error) {
	group, err := h.adminService.GetGroup(c.Request.Context(), groupID)
	if err != nil {
		return nil, err
	}
	if group == nil || group.Status != service.StatusActive || !group.IsExclusive || group.SubscriptionType != "standard" {
		return nil, errExclusiveGroupRequired
	}
	return group, nil
}

func (h *UserHandler) targetUsers(c *gin.Context, req exclusiveGroupBatchRequest) ([]service.User, []int64, error) {
	if req.All && len(req.UserIDs) > 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_TARGET", "all and user_ids cannot be combined")
	}
	if !req.All && len(req.UserIDs) == 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_TARGET", "user_ids or all is required")
	}
	if !req.All && len(req.UserIDs) > maxExclusiveGroupUsers {
		return nil, nil, infraerrors.BadRequest("TOO_MANY_USERS", "too many users")
	}
	filters := service.UserListFilters{
		Status: req.Filters.Status, Role: req.Filters.Role, Search: req.Filters.Search,
		GroupName: req.Filters.GroupName, APIKeyGroupID: req.Filters.APIKeyGroup,
		Attributes: req.Filters.Attributes, VIPTier: req.Filters.VIPTier,
	}
	users := make([]service.User, 0)
	missing := make([]int64, 0)
	if req.All {
		for page := 1; ; page++ {
			items, _, err := h.adminService.ListUsers(c.Request.Context(), page, 500, filters, "id", "asc")
			if err != nil {
				return nil, nil, err
			}
			users = append(users, items...)
			if len(items) < 500 || len(users) > maxExclusiveGroupUsers {
				break
			}
		}
		if len(users) > maxExclusiveGroupUsers {
			return nil, nil, infraerrors.BadRequest("TOO_MANY_USERS", "too many users")
		}
	} else {
		seen := make(map[int64]struct{}, len(req.UserIDs))
		for _, id := range req.UserIDs {
			if id <= 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			user, err := h.adminService.GetUser(c.Request.Context(), id)
			if err != nil {
				if errors.Is(err, service.ErrUserNotFound) {
					missing = append(missing, id)
					continue
				}
				return nil, nil, err
			}
			if user != nil {
				users = append(users, *user)
			} else {
				missing = append(missing, id)
			}
		}
	}
	return users, missing, nil
}

func buildExclusivePreviewToken(req exclusiveGroupBatchRequest, expires time.Time) string {
	copyReq := req
	copyReq.Preview = ""
	data, _ := json.Marshal(struct {
		Request exclusiveGroupBatchRequest `json:"request"`
		Expires int64                      `json:"expires"`
	}{copyReq, expires.Unix()})
	digest := exclusivePreviewDigest(data)
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(expires.Unix(), 10) + ":" + hex.EncodeToString(digest[:])))
}

func validateExclusivePreviewToken(req exclusiveGroupBatchRequest) error {
	decoded, err := base64.RawURLEncoding.DecodeString(req.Preview)
	if err != nil {
		return errPreviewMismatch
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return errPreviewMismatch
	}
	expiresUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().After(time.Unix(expiresUnix, 0)) {
		return errPreviewExpired
	}
	want := buildExclusivePreviewToken(req, time.Unix(expiresUnix, 0))
	if want != req.Preview {
		return errPreviewMismatch
	}
	return nil
}

func (h *UserHandler) PreviewExclusiveGroup(c *gin.Context) {
	var req exclusiveGroupBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid request"))
		return
	}
	group, err := h.validateExclusiveGroup(c, req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	users, missing, err := h.targetUsers(c, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	preview := exclusiveGroupBatchPreview{GroupID: group.ID, GroupName: group.Name, Action: req.Action, RequestedCount: len(users) + len(missing), Eligible: []exclusiveGroupTarget{}, AlreadyGranted: []exclusiveGroupTarget{}, AlreadyRevoked: []exclusiveGroupTarget{}, Skipped: []exclusiveGroupTarget{}, MissingUserIDs: missing}
	for _, user := range users {
		has := containsInt64(user.AllowedGroups, group.ID)
		if req.Action == "grant" {
			switch {
			case user.Status != service.StatusActive:
				preview.Skipped = append(preview.Skipped, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "disabled"})
			case has:
				preview.AlreadyGranted = append(preview.AlreadyGranted, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "already_granted"})
			default:
				preview.Eligible = append(preview.Eligible, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "grant"})
			}
		} else if has {
			preview.Eligible = append(preview.Eligible, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "revoke"})
		} else {
			preview.AlreadyRevoked = append(preview.AlreadyRevoked, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "not_granted"})
		}
	}
	expires := time.Now().Add(exclusiveGroupPreviewTTL)
	preview.ExpiresAt = expires
	req.Preview = ""
	preview.PreviewToken = buildExclusivePreviewToken(req, expires)
	response.Success(c, preview)
}

func (h *UserHandler) ExecuteExclusiveGroup(c *gin.Context) {
	var req exclusiveGroupBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Preview == "" {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "a valid preview token is required"))
		return
	}
	if err := validateExclusivePreviewToken(req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	group, err := h.validateExclusiveGroup(c, req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	users, _, err := h.targetUsers(c, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	executeAdminIdempotentJSON(c, "admin.users.exclusive_groups", req, exclusiveGroupPreviewTTL, func(ctx context.Context) (any, error) {
		result := exclusiveGroupBatchResult{Action: req.Action, Skipped: []exclusiveGroupTarget{}}
		for _, user := range users {
			has := containsInt64(user.AllowedGroups, group.ID)
			var writeErr error
			if req.Action == "grant" {
				if user.Status != service.StatusActive || has {
					continue
				}
				writeErr = h.userService.AddGroupToAllowedGroups(ctx, user.ID, group.ID)
			} else {
				if !has {
					continue
				}
				writeErr = h.userService.RemoveGroupFromUserAllowedGroups(ctx, user.ID, group.ID)
			}
			if writeErr != nil {
				result.Skipped = append(result.Skipped, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "write_failed"})
				continue
			}
			result.Affected++
		}
		return result, nil
	})
}

func (h *UserHandler) PreviewExclusiveGroupCSV(c *gin.Context) {
	groupID, action, raw, hash, err := h.readExclusiveCSV(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	group, err := h.validateExclusiveGroup(c, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	preview, err := h.buildCSVPreview(c, group, action, raw, hash)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"filter_hash": hash, "target_user_id": group.ID, "operation": action, "matched_count": len(preview.Valid)})
	response.Success(c, preview)
}

func (h *UserHandler) ExecuteExclusiveGroupCSV(c *gin.Context) {
	groupID, action, raw, hash, err := h.readExclusiveCSV(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	token := c.PostForm("preview_token")
	if token == "" || c.PostForm("file_sha256") != hash {
		response.ErrorFrom(c, errPreviewMismatch)
		return
	}
	if group, err := h.validateExclusiveGroup(c, groupID); err != nil {
		response.ErrorFrom(c, err)
	} else if err := validateCSVPreviewToken(token, groupID, action, hash); err != nil {
		response.ErrorFrom(c, err)
	} else if preview, err := h.buildCSVPreview(c, group, action, raw, hash); err != nil {
		response.ErrorFrom(c, err)
	} else {
		middleware.SetAuditExtra(c, map[string]any{"filter_hash": hash, "target_user_id": group.ID, "operation": action, "matched_count": len(preview.Valid)})
		executeAdminIdempotentJSON(c, "admin.users.exclusive_groups_csv", map[string]any{"group_id": groupID, "action": action, "file_sha256": hash}, exclusiveGroupPreviewTTL, func(ctx context.Context) (any, error) {
			result := exclusiveGroupBatchResult{Action: action, Skipped: []exclusiveGroupTarget{}}
			for _, item := range preview.Valid {
				var writeErr error
				if action == "grant" {
					writeErr = h.userService.AddGroupToAllowedGroups(ctx, item.UserID, groupID)
				} else {
					writeErr = h.userService.RemoveGroupFromUserAllowedGroups(ctx, item.UserID, groupID)
				}
				if writeErr != nil {
					item.Reason = "write_failed"
					result.Skipped = append(result.Skipped, item)
				} else {
					result.Affected++
				}
			}
			return result, nil
		})
	}
}

func (h *UserHandler) readExclusiveCSV(c *gin.Context) (int64, string, []byte, string, error) {
	groupID, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("group_id")), 10, 64)
	if err != nil || groupID <= 0 {
		return 0, "", nil, "", errCSVInvalid
	}
	action := strings.TrimSpace(c.PostForm("action"))
	if action != "grant" && action != "revoke" {
		return 0, "", nil, "", errCSVInvalid
	}
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		return 0, "", nil, "", errCSVInvalid
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, maxCSVBytes+1))
	if err != nil || len(raw) > maxCSVBytes {
		return 0, "", nil, "", infraerrors.BadRequest("CSV_TOO_LARGE", "the CSV file is too large")
	}
	digest := sha256.Sum256(raw)
	return groupID, action, raw, hex.EncodeToString(digest[:]), nil
}

func (h *UserHandler) buildCSVPreview(c *gin.Context, group *service.Group, action string, raw []byte, hash string) (*csvPreviewResponse, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\ufeff")))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 || len(rows) > maxCSVRows+1 {
		return nil, errCSVInvalid
	}
	headerIndex := -1
	for i, value := range rows[0] {
		if strings.EqualFold(strings.TrimSpace(value), "email") || strings.TrimSpace(value) == "邮箱" {
			headerIndex = i
			break
		}
	}
	if headerIndex < 0 {
		return nil, errCSVInvalid
	}
	preview := &csvPreviewResponse{FileSHA256: hash, GroupID: group.ID, GroupName: group.Name, Action: action, Valid: []exclusiveGroupTarget{}, AlreadyGranted: []exclusiveGroupTarget{}, AlreadyRevoked: []exclusiveGroupTarget{}, DuplicateRows: []int{}, NotFound: []string{}, Disabled: []string{}, Invalid: []string{}}
	seen := map[string]struct{}{}
	for rowIndex, row := range rows[1:] {
		if len(row) <= headerIndex {
			continue
		}
		rawEmail := strings.TrimSpace(row[headerIndex])
		if rawEmail == "" {
			continue
		}
		parsed, parseErr := mail.ParseAddress(rawEmail)
		email := strings.ToLower(strings.TrimSpace(rawEmail))
		if parseErr != nil || parsed.Address != rawEmail || !strings.Contains(email, "@") {
			preview.Invalid = append(preview.Invalid, rawEmail)
			continue
		}
		if _, ok := seen[email]; ok {
			preview.DuplicateRows = append(preview.DuplicateRows, rowIndex+2)
			continue
		}
		seen[email] = struct{}{}
		user, getErr := h.userService.GetByEmail(c.Request.Context(), email)
		if getErr != nil {
			if !errors.Is(getErr, service.ErrUserNotFound) {
				return nil, getErr
			}
			preview.NotFound = append(preview.NotFound, email)
			continue
		}
		if user == nil {
			preview.NotFound = append(preview.NotFound, email)
			continue
		}
		if action == "grant" && user.Status != service.StatusActive {
			preview.Disabled = append(preview.Disabled, email)
			continue
		}
		if containsInt64(user.AllowedGroups, group.ID) {
			if action == "grant" {
				preview.AlreadyGranted = append(preview.AlreadyGranted, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "already_granted"})
			} else {
				preview.Valid = append(preview.Valid, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "revoke"})
			}
			continue
		}
		if action == "grant" {
			preview.Valid = append(preview.Valid, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: action})
		} else {
			preview.AlreadyRevoked = append(preview.AlreadyRevoked, exclusiveGroupTarget{UserID: user.ID, Email: user.Email, Reason: "not_granted"})
		}
	}
	expires := time.Now().Add(exclusiveGroupPreviewTTL)
	preview.ExpiresAt = expires
	preview.PreviewToken = buildCSVPreviewToken(group.ID, action, hash, expires)
	return preview, nil
}

func buildCSVPreviewToken(groupID int64, action, hash string, expires time.Time) string {
	sum := exclusivePreviewDigest([]byte(fmt.Sprintf("%d:%s:%s:%d", groupID, action, hash, expires.Unix())))
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(expires.Unix(), 10) + ":" + hex.EncodeToString(sum[:])))
}

func validateCSVPreviewToken(token string, groupID int64, action, hash string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return errPreviewMismatch
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return errPreviewMismatch
	}
	expiresUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().After(time.Unix(expiresUnix, 0)) {
		return errPreviewExpired
	}
	if buildCSVPreviewToken(groupID, action, hash, time.Unix(expiresUnix, 0)) != token {
		return errPreviewMismatch
	}
	return nil
}

func containsInt64(values []int64, want int64) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
