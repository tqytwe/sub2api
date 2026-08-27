package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	NextChatManagedAPIKeyNamePrefix = "[managed:nextchat]"
	NextChatManagedAPIKeyName       = NextChatManagedAPIKeyNamePrefix + " AI 创作"
	NextChatManagedChatAPIKeyName   = NextChatManagedAPIKeyNamePrefix + " Chat"
	NextChatManagedImageAPIKeyName  = NextChatManagedAPIKeyNamePrefix + " Image"
	NextChatManagedVideoAPIKeyName  = NextChatManagedAPIKeyNamePrefix + " Video"
	NextChatSessionPurposeChat      = "chat"
	NextChatSessionPurposeImage     = "image"
	NextChatSessionPurposeVideo     = "video"
)

type NextChatManagedSession struct {
	UserID  int64  `json:"user_id"`
	APIKey  string `json:"api_key"`
	KeyID   int64  `json:"key_id"`
	Purpose string `json:"purpose,omitempty"`
}

type NextChatManagedSessions struct {
	Chat  NextChatManagedSession `json:"chat"`
	Image NextChatManagedSession `json:"image"`
	Video NextChatManagedSession `json:"video"`
}

type NextChatWorkspaceUser struct {
	ID            int64   `json:"id"`
	Username      string  `json:"username,omitempty"`
	Email         string  `json:"email,omitempty"`
	AvatarURL     string  `json:"avatar_url,omitempty"`
	Role          string  `json:"role,omitempty"`
	IsAdmin       bool    `json:"is_admin"`
	Balance       float64 `json:"balance"`
	FrozenBalance float64 `json:"frozen_balance"`
}

type NextChatWorkspaceAPIKey struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	GroupID       *int64 `json:"group_id,omitempty"`
	GroupName     string `json:"group_name,omitempty"`
	GroupPlatform string `json:"group_platform,omitempty"`
}

type NextChatWorkspaceIdentity struct {
	User   NextChatWorkspaceUser   `json:"user"`
	APIKey NextChatWorkspaceAPIKey `json:"managed_api_key"`
}

type NextChatWorkspaceModel struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	DisplayName          string   `json:"display_name"`
	Platform             string   `json:"platform,omitempty"`
	Channel              string   `json:"channel,omitempty"`
	UseCase              string   `json:"use_case,omitempty"`
	SortOrder            int      `json:"sort_order"`
	EffectiveInputPrice  *float64 `json:"effective_input_price,omitempty"`
	EffectiveOutputPrice *float64 `json:"effective_output_price,omitempty"`
	// ToolCapabilities is derived from the server catalog. It deliberately
	// contains explicit false values so clients do not guess from model names.
	ToolCapabilities ModelToolCapabilities `json:"tool_capabilities"`
	// ImageCapabilities is server-owned. Mobile clients must use this contract
	// for reference-image editing instead of inferring support from model names.
	ImageCapabilities *ImageStudioModelCapabilities `json:"image_capabilities,omitempty"`
	// VideoCapabilities is server-authored. A nil value means this exact model
	// is not approved for mobile video generation.
	VideoCapabilities *VideoModelCapabilities `json:"video_capabilities,omitempty"`
}

type NextChatWorkspaceGroup struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description,omitempty"`
	Platform       string  `json:"platform,omitempty"`
	RateMultiplier float64 `json:"rate_multiplier"`
	SortOrder      int     `json:"sort_order"`
	IsCurrent      bool    `json:"is_current"`
	// LiveAvailable is the group-level authorization result. The gateway still
	// validates the concrete model and upstream session before creating a call.
	LiveAvailable bool `json:"live_available"`
	// VideoAvailable is an explicit group capability. Clients must not infer it
	// from the group name or platform.
	VideoAvailable       bool                     `json:"video_available"`
	VideoUnavailableCode string                   `json:"video_unavailable_code,omitempty"`
	VideoCapabilities    *VideoModelCapabilities  `json:"video_capabilities,omitempty"`
	Models               []NextChatWorkspaceModel `json:"models"`
}

type NextChatWorkspaceModels struct {
	Source                   string                   `json:"source"`
	DefaultModel             string                   `json:"default_model"`
	SelectedGroupID          *int64                   `json:"selected_group_id,omitempty"`
	ImageCapabilitiesVersion string                   `json:"image_capabilities_version,omitempty"`
	VideoCapabilitiesVersion string                   `json:"video_capabilities_version,omitempty"`
	Groups                   []NextChatWorkspaceGroup `json:"groups"`
}

// NextChatImageCapabilitiesVersion is bumped whenever the mobile image
// capability payload changes incompatibly. It lets older clients fail closed.
const NextChatImageCapabilitiesVersion = "2026-07-16.1"

const NextChatVideoCapabilitiesVersion = "2026-08-20.1"

type NextChatAvailableModelResolver interface {
	GetAvailableModels(ctx context.Context, groupID *int64, platform string) []string
}

type NextChatPrompt struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content"`
	Category    string `json:"category,omitempty"`
}

type NextChatPromptCatalog struct {
	ChatPrompts    []NextChatPrompt   `json:"chat_prompts"`
	ImageTemplates ImageStudioCatalog `json:"image_templates"`
}

func IsNextChatManagedAPIKeyName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), NextChatManagedAPIKeyNamePrefix)
}

func (s *SettingService) IsNextChatEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyNextChatEnabled)
	return err == nil && strings.EqualFold(strings.TrimSpace(value), "true")
}

func (s *APIKeyService) IssueNextChatManagedSession(ctx context.Context, userID int64) (*NextChatManagedSession, error) {
	return s.IssueNextChatManagedSessionForPurpose(ctx, userID, NextChatSessionPurposeChat)
}

func (s *APIKeyService) IssueNextChatManagedSessions(ctx context.Context, userID int64) (*NextChatManagedSessions, error) {
	chat, err := s.IssueNextChatManagedSessionForPurpose(ctx, userID, NextChatSessionPurposeChat)
	if err != nil {
		return nil, err
	}
	image, err := s.IssueNextChatManagedSessionForPurpose(ctx, userID, NextChatSessionPurposeImage)
	if err != nil {
		return nil, err
	}
	video, err := s.IssueNextChatManagedSessionForPurpose(ctx, userID, NextChatSessionPurposeVideo)
	if err != nil {
		return nil, err
	}
	return &NextChatManagedSessions{Chat: *chat, Image: *image, Video: *video}, nil
}

func (s *APIKeyService) IssueNextChatManagedSessionForPurpose(ctx context.Context, userID int64, purpose string) (*NextChatManagedSession, error) {
	purpose, keyName, err := normalizeNextChatSessionPurpose(purpose)
	if err != nil {
		return nil, err
	}
	key, err := s.findReusableNextChatManagedKeyForPurpose(ctx, userID, purpose)
	if err != nil {
		return nil, err
	}
	if key != nil {
		groups, groupErr := s.GetNextChatSelectableGroups(ctx, userID)
		if groupErr != nil {
			return nil, groupErr
		}
		if !shouldKeepNextChatManagedKeyGroup(key.GroupID, groups) {
			key, err = s.realignNextChatManagedKeyGroup(ctx, key, userID, pickPreferredNextChatGroupID(groups))
		}
		if err != nil {
			return nil, err
		}
	}
	if key == nil {
		groupID, groupErr := s.pickNextChatGroupID(ctx, userID)
		if groupErr != nil {
			return nil, groupErr
		}
		key, err = s.Create(ctx, userID, CreateAPIKeyRequest{
			Name:    keyName,
			GroupID: groupID,
		})
		if err != nil {
			return nil, err
		}
	}
	return &NextChatManagedSession{
		UserID:  userID,
		APIKey:  key.Key,
		KeyID:   key.ID,
		Purpose: purpose,
	}, nil
}

// IssueNextChatManagedSessionForPurposeAndGroup returns a managed execution
// credential whose group is fixed by its identity. Long-running work such as a
// video job must never borrow the mutable workspace key and then move it to the
// requested group: another worker could otherwise dispatch or bill against the
// wrong group between the update and the upstream call.
func (s *APIKeyService) IssueNextChatManagedSessionForPurposeAndGroup(ctx context.Context, userID int64, purpose string, groupID int64) (*NextChatManagedSession, error) {
	purpose, _, err := normalizeNextChatSessionPurpose(purpose)
	if err != nil {
		return nil, err
	}
	if userID <= 0 || groupID <= 0 {
		return nil, ErrInsufficientPerms
	}

	groups, err := s.GetNextChatSelectableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !nextChatGroupSelectable(groups, groupID) {
		return nil, ErrGroupNotAllowed
	}

	keyName := nextChatManagedSessionGroupKeyName(purpose, groupID)
	key, err := s.findReusableNextChatManagedKeyByName(ctx, userID, keyName)
	if err != nil {
		return nil, err
	}
	if key != nil {
		if key.GroupID == nil || *key.GroupID != groupID {
			// A scoped name bound to a different group is corrupted state. Do not
			// repair it in place because that recreates the cross-job race this API
			// exists to prevent.
			return nil, fmt.Errorf("managed %s execution key is bound to a different group", purpose)
		}
	} else {
		key, err = s.Create(ctx, userID, CreateAPIKeyRequest{
			Name:    keyName,
			GroupID: &groupID,
		})
		if err != nil {
			return nil, err
		}
	}

	return &NextChatManagedSession{
		UserID:  userID,
		APIKey:  key.Key,
		KeyID:   key.ID,
		Purpose: purpose,
	}, nil
}

func (s *APIKeyService) SetNextChatManagedSessionGroup(ctx context.Context, userID int64, purpose string, groupID int64) (*NextChatWorkspaceIdentity, error) {
	purpose, _, err := normalizeNextChatSessionPurpose(purpose)
	if err != nil {
		return nil, err
	}
	session, err := s.IssueNextChatManagedSessionForPurpose(ctx, userID, purpose)
	if err != nil {
		return nil, err
	}
	return s.SetNextChatManagedKeyGroup(ctx, userID, session.KeyID, groupID)
}

func normalizeNextChatSessionPurpose(purpose string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(purpose)) {
	case "", NextChatSessionPurposeChat:
		return NextChatSessionPurposeChat, NextChatManagedAPIKeyName, nil
	case NextChatSessionPurposeImage:
		return NextChatSessionPurposeImage, NextChatManagedImageAPIKeyName, nil
	case NextChatSessionPurposeVideo:
		return NextChatSessionPurposeVideo, NextChatManagedVideoAPIKeyName, nil
	default:
		return "", "", infraerrors.BadRequest("NEXTCHAT_INVALID_SESSION_PURPOSE", "session purpose must be chat, image, or video")
	}
}

func (s *APIKeyService) GetNextChatWorkspaceIdentity(ctx context.Context, userID, apiKeyID int64) (*NextChatWorkspaceIdentity, error) {
	if s == nil || s.userRepo == nil || s.apiKeyRepo == nil {
		return nil, fmt.Errorf("nextchat workspace identity service is not configured")
	}
	if userID <= 0 || apiKeyID <= 0 {
		return nil, ErrInsufficientPerms
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get nextchat user: %w", err)
	}
	key, err := s.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("get nextchat managed api key: %w", err)
	}
	if key.UserID != userID || !IsNextChatManagedAPIKeyName(key.Name) || !key.IsActive() || key.IsExpired() {
		return nil, ErrInsufficientPerms
	}

	group := key.Group
	if group == nil && key.GroupID != nil && *key.GroupID > 0 && s.groupRepo != nil {
		if got, groupErr := s.groupRepo.GetByID(ctx, *key.GroupID); groupErr == nil {
			group = got
		}
	}

	out := &NextChatWorkspaceIdentity{
		User: NextChatWorkspaceUser{
			ID:            user.ID,
			Username:      user.Username,
			Email:         user.Email,
			AvatarURL:     user.AvatarURL,
			Role:          user.Role,
			IsAdmin:       user.IsAdmin(),
			Balance:       user.Balance,
			FrozenBalance: user.FrozenBalance,
		},
		APIKey: NextChatWorkspaceAPIKey{
			ID:      key.ID,
			Name:    key.Name,
			GroupID: key.GroupID,
		},
	}
	if group != nil {
		out.APIKey.GroupName = group.Name
		out.APIKey.GroupPlatform = group.Platform
	}
	return out, nil
}

func (s *APIKeyService) SetNextChatManagedKeyGroup(ctx context.Context, userID, apiKeyID, groupID int64) (*NextChatWorkspaceIdentity, error) {
	if s == nil || s.apiKeyRepo == nil {
		return nil, fmt.Errorf("nextchat managed key service is not configured")
	}
	if userID <= 0 || apiKeyID <= 0 || groupID <= 0 {
		return nil, ErrInsufficientPerms
	}
	key, err := s.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("get nextchat managed api key: %w", err)
	}
	if key.UserID != userID || !IsNextChatManagedAPIKeyName(key.Name) || !key.IsActive() || key.IsExpired() {
		return nil, ErrInsufficientPerms
	}
	groups, err := s.GetNextChatSelectableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, group := range groups {
		if group.ID == groupID {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrInsufficientPerms
	}

	key, err = s.realignNextChatManagedKeyGroup(ctx, key, userID, &groupID)
	if err != nil {
		return nil, err
	}
	return s.GetNextChatWorkspaceIdentity(ctx, userID, key.ID)
}

func BuildNextChatPromptCatalog() NextChatPromptCatalog {
	return NextChatPromptCatalog{
		ChatPrompts: []NextChatPrompt{
			{
				ID:          "general-assistant",
				Title:       "通用助手",
				Description: "日常问答、写作、分析和总结",
				Content:     "你是极速蹬 AI 工作台里的专业助手。请用清晰、准确、可执行的方式回答用户问题。",
				Category:    "chat",
			},
			{
				ID:          "ecommerce-copy",
				Title:       "电商文案",
				Description: "商品卖点、标题、详情页和投放文案",
				Content:     "请根据用户给出的商品信息，输出适合电商场景的标题、核心卖点、详情页结构和可直接使用的营销文案。",
				Category:    "ecommerce",
			},
		},
		ImageTemplates: defaultImageStudioCatalog(),
	}
}

func BuildNextChatPromptCatalogFromPublicPrompts(prompts []PublicPrompt) NextChatPromptCatalog {
	catalog := BuildNextChatPromptCatalog()
	chatPrompts := make([]NextChatPrompt, 0, len(prompts))
	for _, prompt := range prompts {
		nextChatPrompt, ok := nextChatPromptFromPublicPrompt(prompt)
		if ok {
			chatPrompts = append(chatPrompts, nextChatPrompt)
		}
	}
	if len(chatPrompts) > 0 {
		catalog.ChatPrompts = chatPrompts
	}
	return catalog
}

func nextChatPromptFromPublicPrompt(prompt PublicPrompt) (NextChatPrompt, bool) {
	if IsNextChatPublicImagePrompt(prompt) {
		return NextChatPrompt{}, false
	}
	content := strings.TrimSpace(prompt.PromptText)
	title := strings.TrimSpace(prompt.Title)
	if content == "" || title == "" {
		return NextChatPrompt{}, false
	}
	id := fmt.Sprintf("prompt-%d", prompt.ID)
	if prompt.Version > 0 {
		id = fmt.Sprintf("%s-v%d", id, prompt.Version)
	}
	category := strings.TrimSpace(prompt.Purpose)
	if category == "" {
		category = strings.TrimSpace(prompt.Subject)
	}
	if category == "" {
		category = strings.TrimSpace(prompt.Style)
	}
	return NextChatPrompt{
		ID:          id,
		Title:       title,
		Description: strings.TrimSpace(prompt.Description),
		Content:     content,
		Category:    category,
	}, true
}

func IsNextChatPublicImagePrompt(prompt PublicPrompt) bool {
	if len(prompt.Sizes) > 0 || prompt.RequiresReference {
		return true
	}
	if requirement := strings.TrimSpace(string(prompt.ReferenceRequirement)); requirement != "" && requirement != string(PromptReferenceNone) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(prompt.Purpose)) {
	case "image", "image_studio", "image-studio":
		return true
	}
	for _, model := range prompt.Models {
		if _, ok := ResolveImageStudioModelCapability(model); ok {
			return true
		}
	}
	return false
}

func NextChatImagePromptModelIDs() []string {
	out := make([]string, 0, len(imageStudioModelPreference))
	for _, model := range imageStudioModelPreference {
		if _, ok := ResolveImageStudioModelCapability(model); ok {
			out = append(out, model)
		}
	}
	return out
}

func NextChatImagePromptModelLikePatterns() []string {
	return []string{
		"gpt-image-%",
		"models/gpt-image-%",
		"grok-imagine%",
		"models/grok-imagine%",
		"imagen-%",
		"imagen_%",
		"models/imagen-%",
		"models/imagen_%",
		"%image%",
		"%dall-e%",
		"%stable-diffusion%",
		"%sdxl%",
		"%flux%",
		"%recraft%",
		"%midjourney%",
	}
}

func (s *APIKeyService) findReusableNextChatManagedKeyForPurpose(ctx context.Context, userID int64, purpose string) (*APIKey, error) {
	if s == nil || s.apiKeyRepo == nil {
		return nil, fmt.Errorf("api key service is not configured")
	}
	if lister, ok := s.apiKeyRepo.(apiKeyAllByUserIDLister); ok {
		keys, err := lister.ListAllByUserID(ctx, userID, APIKeyListFilters{Status: StatusActive})
		if err != nil {
			return nil, fmt.Errorf("list managed api keys: %w", err)
		}
		for i := range keys {
			if nextChatManagedKeyMatchesPurpose(keys[i].Name, purpose) && keys[i].IsActive() && !keys[i].IsExpired() {
				return &keys[i], nil
			}
		}
		return nil, nil
	}

	keys, _, err := s.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 100}, APIKeyListFilters{
		Search: NextChatManagedAPIKeyNamePrefix,
		Status: StatusActive,
	})
	if err != nil {
		return nil, fmt.Errorf("search managed api keys: %w", err)
	}
	for i := range keys {
		if nextChatManagedKeyMatchesPurpose(keys[i].Name, purpose) && keys[i].IsActive() && !keys[i].IsExpired() {
			return &keys[i], nil
		}
	}
	return nil, nil
}

func nextChatManagedKeyMatchesPurpose(name, purpose string) bool {
	name = strings.TrimSpace(name)
	switch purpose {
	case NextChatSessionPurposeChat:
		return name == NextChatManagedAPIKeyName || name == NextChatManagedChatAPIKeyName
	case NextChatSessionPurposeImage:
		return name == NextChatManagedImageAPIKeyName
	case NextChatSessionPurposeVideo:
		return name == NextChatManagedVideoAPIKeyName
	default:
		return false
	}
}

func nextChatManagedSessionGroupKeyName(purpose string, groupID int64) string {
	label := "Chat"
	switch purpose {
	case NextChatSessionPurposeImage:
		label = "Image"
	case NextChatSessionPurposeVideo:
		label = "Video"
	}
	return fmt.Sprintf("%s %s Group %d", NextChatManagedAPIKeyNamePrefix, label, groupID)
}

func nextChatGroupSelectable(groups []Group, groupID int64) bool {
	for _, group := range groups {
		if group.ID == groupID {
			return true
		}
	}
	return false
}

func (s *APIKeyService) findReusableNextChatManagedKeyByName(ctx context.Context, userID int64, name string) (*APIKey, error) {
	if s == nil || s.apiKeyRepo == nil {
		return nil, fmt.Errorf("api key service is not configured")
	}
	if lister, ok := s.apiKeyRepo.(apiKeyAllByUserIDLister); ok {
		keys, err := lister.ListAllByUserID(ctx, userID, APIKeyListFilters{Status: StatusActive})
		if err != nil {
			return nil, fmt.Errorf("list managed api keys: %w", err)
		}
		for i := range keys {
			if strings.TrimSpace(keys[i].Name) == name && keys[i].IsActive() && !keys[i].IsExpired() {
				return &keys[i], nil
			}
		}
		return nil, nil
	}

	keys, _, err := s.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 100}, APIKeyListFilters{
		Search: name,
		Status: StatusActive,
	})
	if err != nil {
		return nil, fmt.Errorf("search managed api keys: %w", err)
	}
	for i := range keys {
		if strings.TrimSpace(keys[i].Name) == name && keys[i].IsActive() && !keys[i].IsExpired() {
			return &keys[i], nil
		}
	}
	return nil, nil
}

func (s *APIKeyService) pickNextChatGroupID(ctx context.Context, userID int64) (*int64, error) {
	// Preserve the historical default for users who already have ordinary API
	// keys: the managed key starts in one of those billing groups. The Canvas
	// video workspace can then explicitly switch it to a permitted video group.
	ownedGroups, err := s.getNextChatUserOwnedKeyGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.userRepo != nil && s.groupRepo != nil && s.userSubRepo != nil {
		// Ordinary keys can outlive a subscription or an explicit group grant.
		// Filter them against the current permission result before using one as
		// the managed session's initial billing group.
		available, availableErr := s.GetAvailableGroups(ctx, userID)
		if availableErr != nil {
			return nil, availableErr
		}
		availableByID := make(map[int64]struct{}, len(available))
		for _, group := range available {
			availableByID[group.ID] = struct{}{}
		}
		authorizedOwned := make([]Group, 0, len(ownedGroups))
		for _, group := range ownedGroups {
			if _, ok := availableByID[group.ID]; ok {
				authorizedOwned = append(authorizedOwned, group)
			}
		}
		if len(authorizedOwned) > 0 {
			return pickPreferredNextChatGroupID(authorizedOwned), nil
		}
		return pickPreferredNextChatGroupID(available), nil
	}
	if len(ownedGroups) > 0 {
		return pickPreferredNextChatGroupID(ownedGroups), nil
	}
	groups, err := s.GetNextChatSelectableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	return pickPreferredNextChatGroupID(groups), nil
}

func (s *APIKeyService) GetNextChatSelectableGroups(ctx context.Context, userID int64) ([]Group, error) {
	groups, err := s.getNextChatUserOwnedKeyGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Managed workspaces must expose every group the user can bind, even when
	// an existing ordinary API key already belongs to another group. Otherwise
	// a text-only key group can hide a video group that has not been bound yet.
	if s.userRepo != nil && s.groupRepo != nil && s.userSubRepo != nil {
		available, availableErr := s.GetAvailableGroups(ctx, userID)
		if availableErr != nil {
			// Preserve already-bound groups if the optional permission lookup is
			// unavailable; switching to a group still requires an existing key or
			// a successful permission lookup.
			if len(groups) == 0 {
				return nil, availableErr
			}
		} else {
			byID := make(map[int64]Group, len(groups)+len(available))
			// An existing ordinary key is only selectable while the user can
			// currently bind its group. This prevents expired exclusive groups
			// from being carried into a new managed session.
			for _, group := range available {
				byID[group.ID] = group
			}
			for _, group := range groups {
				if _, authorized := byID[group.ID]; authorized {
					byID[group.ID] = group
				}
			}
			groups = groups[:0]
			for _, group := range byID {
				groups = append(groups, group)
			}
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].ID < groups[j].ID
	})
	return groups, nil
}

func (s *APIKeyService) getNextChatUserOwnedKeyGroups(ctx context.Context, userID int64) ([]Group, error) {
	lister, ok := s.apiKeyRepo.(apiKeyAllByUserIDLister)
	if !ok {
		return nil, nil
	}
	keys, err := lister.ListAllByUserID(ctx, userID, APIKeyListFilters{Status: StatusActive})
	if err != nil {
		return nil, fmt.Errorf("list user api key groups: %w", err)
	}
	groupByID := make(map[int64]Group)
	missingGroupIDs := make(map[int64]struct{})
	for i := range keys {
		key := &keys[i]
		if IsNextChatManagedAPIKeyName(key.Name) || key.GroupID == nil || *key.GroupID <= 0 {
			continue
		}
		if key.Group != nil && key.Group.ID == *key.GroupID && key.Group.IsActive() {
			groupByID[*key.GroupID] = *key.Group
			continue
		}
		missingGroupIDs[*key.GroupID] = struct{}{}
	}
	if len(missingGroupIDs) > 0 && s.groupRepo != nil {
		groups, listErr := s.groupRepo.ListActive(ctx)
		if listErr != nil {
			return nil, fmt.Errorf("list active groups: %w", listErr)
		}
		for _, group := range groups {
			if _, needed := missingGroupIDs[group.ID]; needed {
				groupByID[group.ID] = group
			}
		}
	}
	out := make([]Group, 0, len(groupByID))
	for _, group := range groupByID {
		out = append(out, group)
	}
	return out, nil
}

func (s *ModelCatalogService) GetNextChatWorkspaceModels(ctx context.Context, userID, apiKeyID int64) (*NextChatWorkspaceModels, error) {
	if s == nil || s.apiKeyService == nil {
		return nil, fmt.Errorf("nextchat workspace model service is not configured")
	}
	if userID <= 0 || apiKeyID <= 0 {
		return nil, ErrInsufficientPerms
	}

	identity, err := s.apiKeyService.GetNextChatWorkspaceIdentity(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	selectableGroups, err := s.apiKeyService.GetNextChatSelectableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	pricing, err := s.ListMyPricing(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := &NextChatWorkspaceModels{
		Source:                   "/v1/models",
		SelectedGroupID:          identity.APIKey.GroupID,
		ImageCapabilitiesVersion: NextChatImageCapabilitiesVersion,
		VideoCapabilitiesVersion: NextChatVideoCapabilitiesVersion,
		Groups:                   make([]NextChatWorkspaceGroup, 0, len(selectableGroups)),
	}
	for _, group := range selectableGroups {
		g := NextChatWorkspaceGroup{
			ID:             group.ID,
			Name:           group.Name,
			Description:    group.Description,
			Platform:       group.Platform,
			RateMultiplier: group.RateMultiplier,
			SortOrder:      group.SortOrder,
			IsCurrent:      identity.APIKey.GroupID != nil && *identity.APIKey.GroupID == group.ID,
			LiveAvailable:  group.Platform == PlatformOpenAI && group.AllowLive,
			VideoAvailable: group.HasVideoGenerationCapability(),
			Models:         []NextChatWorkspaceModel{},
		}
		if !g.VideoAvailable {
			g.VideoUnavailableCode = "VIDEO_GROUP_UNAVAILABLE"
		}
		out.Groups = append(out.Groups, g)
	}

	metadata := nextChatWorkspaceModelMetadata{}
	if pricing != nil {
		metadata = buildNextChatWorkspaceModelMetadata(pricing.Models)
	}
	toolCapabilityEntries := []SiteModelCatalogEntry{}
	if s.repo != nil {
		visible := true
		if entries, listErr := s.repo.ListCatalog(ctx, CatalogListFilter{VisibleAuth: &visible}); listErr == nil {
			toolCapabilityEntries = entries
		}
	}
	for groupIndex := range out.Groups {
		group := &out.Groups[groupIndex]
		sourceGroup := selectableGroups[groupIndex]
		availableModels := []string{}
		if s.modelResolver != nil {
			availableModels = s.modelResolver.GetAvailableModels(ctx, &group.ID, group.Platform)
		}
		availableModels = filterNextChatWorkspaceModelsForGroup(sourceGroup, availableModels)
		seen := make(map[string]struct{}, len(availableModels))
		for _, modelID := range availableModels {
			modelID = strings.TrimSpace(modelID)
			if modelID == "" {
				continue
			}
			normalizedModelID := strings.ToLower(modelID)
			if _, exists := seen[normalizedModelID]; exists {
				continue
			}
			seen[normalizedModelID] = struct{}{}
			meta := metadata.lookup(group.ID, group.Platform, modelID)
			workspaceModel := buildNextChatWorkspaceModel(group.Platform, modelID, meta)
			upstreamCapabilities := ModelToolCapabilities{}
			if s.pricingService != nil {
				upstreamCapabilities = s.pricingService.GetModelToolCapabilities(modelID)
			}
			workspaceModel.ToolCapabilities = resolveNextChatWorkspaceModelToolCapabilities(
				toolCapabilityEntries,
				sourceGroup,
				modelID,
				upstreamCapabilities,
			)
			// Video authorization is group- and model-scoped. Publish the
			// server-owned capability contract only for models that have an
			// explicit video price; clients must not infer support from names.
			if capability, ok := ResolveVideoModelCapabilities(sourceGroup, group.Platform, modelID); ok {
				workspaceModel.VideoCapabilities = &capability
			}
			group.Models = append(group.Models, workspaceModel)
		}
	}

	for i := range out.Groups {
		videoModels := 0
		for modelIndex := range out.Groups[i].Models {
			if out.Groups[i].Models[modelIndex].VideoCapabilities != nil {
				videoModels++
			}
		}
		if videoModels == 0 {
			out.Groups[i].VideoAvailable = false
			out.Groups[i].VideoUnavailableCode = "VIDEO_MODEL_UNAVAILABLE"
		} else {
			out.Groups[i].VideoAvailable = true
			out.Groups[i].VideoUnavailableCode = ""
			for modelIndex := range out.Groups[i].Models {
				if capability := out.Groups[i].Models[modelIndex].VideoCapabilities; capability != nil {
					copy := *capability
					copy.Operations = append([]string(nil), capability.Operations...)
					copy.SupportedResolutions = append([]string(nil), capability.SupportedResolutions...)
					copy.SupportedRatios = append([]string(nil), capability.SupportedRatios...)
					copy.SupportedDurations = append([]int(nil), capability.SupportedDurations...)
					out.Groups[i].VideoCapabilities = &copy
					break
				}
			}
		}
		sort.SliceStable(out.Groups[i].Models, func(a, b int) bool {
			left, right := out.Groups[i].Models[a], out.Groups[i].Models[b]
			if left.SortOrder != right.SortOrder {
				return left.SortOrder < right.SortOrder
			}
			return left.Name < right.Name
		})
		if out.DefaultModel == "" && out.Groups[i].IsCurrent && len(out.Groups[i].Models) > 0 {
			out.DefaultModel = out.Groups[i].Models[0].Name
		}
	}

	return out, nil
}

func filterNextChatWorkspaceModelsForGroup(group Group, availableModels []string) []string {
	if !group.ModelsListConfig.Enabled {
		return availableModels
	}
	if len(availableModels) == 0 {
		return nil
	}
	allowed := make([]string, 0, len(availableModels))
	for _, model := range availableModels {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		allowed = append(allowed, model)
	}
	if len(allowed) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(group.ModelsListConfig.Models))
	out := make([]string, 0, len(group.ModelsListConfig.Models))
	for _, model := range group.ModelsListConfig.Models {
		model = strings.TrimSpace(model)
		if model == "" || !nextChatWorkspaceModelAllowedByPatterns(allowed, model) {
			continue
		}
		seenKey := strings.ToLower(model)
		if _, ok := seen[seenKey]; ok {
			continue
		}
		seen[seenKey] = struct{}{}
		out = append(out, model)
	}
	return out
}

func nextChatWorkspaceModelAllowedByPatterns(availablePatterns []string, model string) bool {
	for _, pattern := range availablePatterns {
		if pattern == model {
			return true
		}
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}

type nextChatWorkspaceModelMetadata struct {
	byGroupModel map[nextChatWorkspaceGroupModelKey]nextChatWorkspaceModelMeta
}

type nextChatWorkspaceGroupModelKey struct {
	groupID int64
	model   string
}

type nextChatWorkspaceModelMeta struct {
	platform             string
	channel              string
	useCase              string
	sortOrder            int
	effectiveInputPrice  *float64
	effectiveOutputPrice *float64
}

func buildNextChatWorkspaceModelMetadata(rows []MyModelPricingRow) nextChatWorkspaceModelMetadata {
	metadata := nextChatWorkspaceModelMetadata{
		byGroupModel: make(map[nextChatWorkspaceGroupModelKey]nextChatWorkspaceModelMeta),
	}
	for _, row := range rows {
		model := strings.ToLower(strings.TrimSpace(row.Name))
		if model == "" {
			continue
		}
		meta := nextChatWorkspaceModelMeta{
			platform:             row.Platform,
			channel:              row.Channel,
			useCase:              row.UseCase,
			sortOrder:            row.SortOrder,
			effectiveInputPrice:  row.EffectiveInputPrice,
			effectiveOutputPrice: row.EffectiveOutputPrice,
		}
		for _, group := range row.Groups {
			metadata.byGroupModel[nextChatWorkspaceGroupModelKey{groupID: group.ID, model: model}] = meta
		}
	}
	return metadata
}

func (m nextChatWorkspaceModelMetadata) lookup(groupID int64, groupPlatform, modelID string) nextChatWorkspaceModelMeta {
	model := strings.ToLower(strings.TrimSpace(modelID))
	if meta, ok := m.byGroupModel[nextChatWorkspaceGroupModelKey{groupID: groupID, model: model}]; ok {
		if nextChatModelPlatformMatchesGroup(meta.platform, groupPlatform) {
			return meta
		}
	}
	return nextChatWorkspaceModelMeta{}
}

func buildNextChatWorkspaceModel(groupPlatform, modelID string, meta nextChatWorkspaceModelMeta) NextChatWorkspaceModel {
	platform := strings.TrimSpace(groupPlatform)
	if platform == "" {
		platform = meta.platform
	}
	displayName := modelID
	if platform != "" {
		displayName = fmt.Sprintf("%s · %s", modelID, platform)
	}
	model := NextChatWorkspaceModel{
		ID:                   modelID,
		Name:                 modelID,
		DisplayName:          displayName,
		Platform:             platform,
		Channel:              meta.channel,
		UseCase:              meta.useCase,
		SortOrder:            meta.sortOrder,
		EffectiveInputPrice:  meta.effectiveInputPrice,
		EffectiveOutputPrice: meta.effectiveOutputPrice,
	}
	if capability, ok := ResolveImageStudioProviderCapability(platform, modelID); ok {
		model.ImageCapabilities = &capability
	}
	return model
}

// resolveNextChatModelToolCapabilities applies the only permitted capability
// precedence: explicit catalog values, then exact upstream metadata. Platform
// web search is a function tool backed by Exa/DuckDuckGo, not an upstream's
// native search product, so every verified function-calling model is eligible
// unless the catalog explicitly denies it. It never performs a model-name
// fallback, so private aliases must be declared by an administrator before a
// mobile client can use a tool.
func resolveNextChatModelToolCapabilities(overrides ModelToolCapabilityOverrides, upstream ModelToolCapabilities) ModelToolCapabilities {
	resolved := upstream
	if overrides.FunctionCalling != nil {
		resolved.FunctionCalling = *overrides.FunctionCalling
	}
	if overrides.ToolChoice != nil {
		resolved.ToolChoice = *overrides.ToolChoice
	}
	if overrides.WebSearch != nil {
		resolved.WebSearch = *overrides.WebSearch
	} else {
		resolved.WebSearch = resolved.FunctionCalling
	}
	// No catalog override can make a model execute a function it cannot call.
	if !resolved.FunctionCalling {
		resolved.WebSearch = false
	}
	if overrides.Live != nil {
		resolved.Live = *overrides.Live
	}
	return resolved
}

// resolveNextChatWorkspaceModelToolCapabilities finds a server-owned catalog
// declaration for the exact visible model. A declaration must match the
// selected group (including an explicit group scope); otherwise it is ignored.
// This is what lets an administrator approve a private alias without letting a
// same-named model in another group inherit that approval.
func resolveNextChatWorkspaceModelToolCapabilities(
	entries []SiteModelCatalogEntry,
	group Group,
	modelID string,
	upstream ModelToolCapabilities,
) ModelToolCapabilities {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ModelToolCapabilities{}
	}

	var generic *SiteModelCatalogEntry
	for index := range entries {
		entry := &entries[index]
		if !strings.EqualFold(strings.TrimSpace(entry.ModelName), modelID) ||
			!catalogAllowsGroup(*entry, group.ID, group.Platform) {
			continue
		}
		if normalizeNextChatModelPlatform(entry.Platform) == normalizeNextChatModelPlatform(group.Platform) {
			return resolveNextChatModelToolCapabilities(entry.ToolCapabilities, upstream)
		}
		if strings.TrimSpace(entry.Platform) == "" {
			generic = entry
		}
	}
	if generic != nil {
		return resolveNextChatModelToolCapabilities(generic.ToolCapabilities, upstream)
	}
	return resolveNextChatModelToolCapabilities(ModelToolCapabilityOverrides{}, upstream)
}

func nextChatModelPlatformMatchesGroup(modelPlatform, groupPlatform string) bool {
	model := normalizeNextChatModelPlatform(modelPlatform)
	group := normalizeNextChatModelPlatform(groupPlatform)
	if model == "" || group == "" {
		return true
	}
	if model == group {
		return true
	}
	return group == PlatformAntigravity && (model == PlatformAnthropic || model == PlatformGemini)
}

func normalizeNextChatModelPlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "", "—":
		return ""
	case "openai":
		return PlatformOpenAI
	case "anthropic", "claude":
		return PlatformAnthropic
	case "gemini", "google":
		return PlatformGemini
	case "grok", "xai":
		return PlatformGrok
	case "antigravity":
		return PlatformAntigravity
	default:
		return strings.ToLower(strings.TrimSpace(platform))
	}
}

func shouldKeepNextChatManagedKeyGroup(groupID *int64, groups []Group) bool {
	if groupID == nil {
		return len(groups) == 0
	}
	if len(groups) == 0 {
		return false
	}
	for _, group := range groups {
		if group.ID == *groupID {
			return true
		}
	}
	return false
}

func (s *APIKeyService) realignNextChatManagedKeyGroup(ctx context.Context, key *APIKey, userID int64, groupID *int64) (*APIKey, error) {
	if key == nil || sameAPIKeyGroupID(key.GroupID, groupID) {
		return key, nil
	}
	if key.UserID != userID {
		return nil, ErrInsufficientPerms
	}

	updated := *key
	updated.GroupID = groupID
	if err := s.apiKeyRepo.Update(ctx, &updated, APIKeyUpdateFields{GroupID: true}); err != nil {
		return nil, fmt.Errorf("realign nextchat managed api key group: %w", err)
	}
	s.InvalidateAuthCacheByKey(ctx, updated.Key)
	s.compileAPIKeyIPRules(&updated)
	return &updated, nil
}

func pickPreferredNextChatGroupID(groups []Group) *int64 {
	if len(groups) == 0 {
		return nil
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].ID < groups[j].ID
	})
	for i := range groups {
		switch strings.ToLower(strings.TrimSpace(groups[i].Platform)) {
		case PlatformOpenAI, PlatformGrok:
			id := groups[i].ID
			return &id
		}
	}
	id := groups[0].ID
	return &id
}

func sameAPIKeyGroupID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
