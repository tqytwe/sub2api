package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
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
	// NextChatGroupPinnedSessionBinding is returned only for replacement
	// sessions whose key name and database group are immutable as a pair.
	// Clients must not treat an older, mutable managed key as equivalent.
	NextChatGroupPinnedSessionBinding = "group-pinned-v1"
)

type NextChatManagedSession struct {
	UserID  int64  `json:"user_id"`
	APIKey  string `json:"api_key"`
	KeyID   int64  `json:"key_id"`
	Purpose string `json:"purpose,omitempty"`
	GroupID *int64 `json:"group_id,omitempty"`
	Binding string `json:"binding,omitempty"`
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Platform    string `json:"platform,omitempty"`
	// Modalities and the media fields are the server-owned contract shared by
	// Canvas, the mobile client, and the OpenAI-compatible model list.  Legacy
	// chat rows intentionally omit them; clients must not infer a media mode
	// from an omitted declaration.
	Modalities           []string `json:"modalities,omitempty"`
	Adapter              string   `json:"adapter,omitempty"`
	CapabilityVersion    string   `json:"capability_version,omitempty"`
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
	ImageCapabilities *ModelImageCapabilities `json:"image_capabilities,omitempty"`
	// VideoCapabilities is only present for an executable, catalog-declared
	// video model.  It is deliberately distinct from ImageCapabilities so a
	// shared model ID cannot borrow another purpose's behavior.
	VideoCapabilities *MobileVideoCapabilities `json:"video_capabilities,omitempty"`
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
	// VideoAvailable is a summary of the exact model declarations below. It is
	// false until at least one schedulable model has an executable video
	// declaration; a group name or a model-name pattern is never enough.
	VideoAvailable bool                     `json:"video_available"`
	Models         []NextChatWorkspaceModel `json:"models"`
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
const NextChatImageCapabilitiesVersion = "2026-08-26.1"

// NextChatVideoCapabilitiesVersion changes when the server-owned video
// workspace shape changes. It lets managed clients fail closed on a contract
// they cannot safely interpret.
const NextChatVideoCapabilitiesVersion = MobileVideoCapabilitiesVersion

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
		if shouldKeepNextChatManagedKeyGroup(key.GroupID, groups) {
			return nextChatManagedSessionFromKey(key, userID, purpose), nil
		}
	}

	groupID, groupErr := s.pickNextChatGroupID(ctx, userID)
	if groupErr != nil {
		return nil, groupErr
	}
	if groupID != nil && *groupID > 0 {
		return s.IssueNextChatManagedSessionForPurposeAndGroup(ctx, userID, purpose, *groupID)
	}
	key, err = s.Create(ctx, userID, CreateAPIKeyRequest{Name: keyName})
	if err != nil {
		return nil, err
	}
	return nextChatManagedSessionFromKey(key, userID, purpose), nil
}

func (s *APIKeyService) SetNextChatManagedSessionGroup(ctx context.Context, userID int64, purpose string, groupID int64) (*NextChatWorkspaceIdentity, error) {
	purpose, _, err := normalizeNextChatSessionPurpose(purpose)
	if err != nil {
		return nil, err
	}
	// This method remains only for the legacy generic group-switch route. The
	// modern Canvas/mobile paths use SwitchNextChatManagedSessionGroup or the
	// explicit group-pinned issuer below and receive a replacement credential.
	// Preserve old clients while refusing to mutate a new scoped key.
	session, err := s.IssueNextChatManagedSessionForPurpose(ctx, userID, purpose)
	if err != nil {
		return nil, err
	}
	return s.SetNextChatManagedKeyGroup(ctx, userID, session.KeyID, groupID)
}

// SwitchNextChatManagedSessionGroup verifies the caller's current managed
// session and returns a new immutable purpose/group-bound session. It never
// changes GroupID on an existing key, so concurrent Canvas tabs and devices
// cannot reroute or bill one another's request against a different group.
func (s *APIKeyService) SwitchNextChatManagedSessionGroup(
	ctx context.Context,
	userID int64,
	currentKeyID int64,
	purpose string,
	groupID int64,
) (*NextChatManagedSession, *NextChatWorkspaceIdentity, error) {
	purpose, _, err := normalizeNextChatSessionPurpose(purpose)
	if err != nil {
		return nil, nil, err
	}
	if userID <= 0 || currentKeyID <= 0 {
		return nil, nil, ErrInsufficientPerms
	}
	current, err := s.GetByID(ctx, currentKeyID)
	if err != nil {
		return nil, nil, fmt.Errorf("get nextchat managed api key: %w", err)
	}
	if current.UserID != userID || !current.IsActive() || current.IsExpired() || !nextChatManagedKeyMatchesPurpose(current.Name, purpose) {
		return nil, nil, ErrInsufficientPerms
	}
	session, err := s.IssueNextChatManagedSessionForPurposeAndGroup(ctx, userID, purpose, groupID)
	if err != nil {
		return nil, nil, err
	}
	identity, err := s.GetNextChatWorkspaceIdentity(ctx, userID, session.KeyID)
	if err != nil {
		return nil, nil, err
	}
	return session, identity, nil
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

func nextChatManagedSessionFromKey(key *APIKey, userID int64, purpose string) *NextChatManagedSession {
	if key == nil {
		return nil
	}
	var groupID *int64
	if key.GroupID != nil {
		value := *key.GroupID
		groupID = &value
	}
	session := &NextChatManagedSession{
		UserID:  userID,
		APIKey:  key.Key,
		KeyID:   key.ID,
		Purpose: purpose,
		GroupID: groupID,
	}
	if groupID != nil && isNextChatManagedScopedAPIKeyName(key.Name) {
		session.Binding = NextChatGroupPinnedSessionBinding
	}
	return session
}

func nextChatManagedScopedAPIKeyName(purpose string, groupID int64) (string, error) {
	if groupID <= 0 {
		return "", ErrInsufficientPerms
	}
	switch purpose {
	case NextChatSessionPurposeChat:
		return NextChatManagedChatAPIKeyName + "/" + strconv.FormatInt(groupID, 10), nil
	case NextChatSessionPurposeImage:
		return NextChatManagedImageAPIKeyName + "/" + strconv.FormatInt(groupID, 10), nil
	case NextChatSessionPurposeVideo:
		return NextChatManagedVideoAPIKeyName + "/" + strconv.FormatInt(groupID, 10), nil
	default:
		return "", infraerrors.BadRequest("NEXTCHAT_INVALID_SESSION_PURPOSE", "session purpose must be chat, image, or video")
	}
}

func isNextChatManagedScopedAPIKeyName(name string) bool {
	name = strings.TrimSpace(name)
	return strings.HasPrefix(name, NextChatManagedChatAPIKeyName+"/") ||
		strings.HasPrefix(name, NextChatManagedImageAPIKeyName+"/") ||
		strings.HasPrefix(name, NextChatManagedVideoAPIKeyName+"/")
}

// IssueNextChatManagedSessionForPurposeAndGroup creates or reuses an immutable
// key for one exact purpose and group. A scoped key is intentionally separate
// from the historical single key per purpose: rebinding that legacy key is a
// cross-request race and can apply the wrong routing or billing group.
func (s *APIKeyService) IssueNextChatManagedSessionForPurposeAndGroup(ctx context.Context, userID int64, purpose string, groupID int64) (*NextChatManagedSession, error) {
	purpose, _, err := normalizeNextChatSessionPurpose(purpose)
	if err != nil {
		return nil, err
	}
	if s == nil || s.apiKeyRepo == nil || userID <= 0 || groupID <= 0 {
		return nil, ErrInsufficientPerms
	}
	groups, err := s.GetNextChatSelectableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !nextChatGroupIsSelectable(groups, groupID) {
		return nil, ErrGroupNotAllowed
	}
	keyName, err := nextChatManagedScopedAPIKeyName(purpose, groupID)
	if err != nil {
		return nil, err
	}
	key, err := s.findReusableNextChatManagedKeyByNameAndGroup(ctx, userID, keyName, groupID)
	if err != nil {
		return nil, err
	}
	if key == nil {
		key, err = s.Create(ctx, userID, CreateAPIKeyRequest{Name: keyName, GroupID: &groupID})
		if err != nil {
			return nil, err
		}
	}
	return nextChatManagedSessionFromKey(key, userID, purpose), nil
}

func nextChatGroupIsSelectable(groups []Group, groupID int64) bool {
	for _, group := range groups {
		if group.ID == groupID {
			return true
		}
	}
	return false
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
	if isNextChatManagedScopedAPIKeyName(key.Name) {
		// Fixed keys may only be changed through the replacement-session
		// contract.  Updating their GroupID would restore the multi-tab race
		// this namespace exists to eliminate.
		return nil, ErrInsufficientPerms
	}
	if _, ok := nextChatManagedKeyPurpose(key.Name); !ok {
		return nil, ErrInsufficientPerms
	}
	groups, err := s.GetNextChatSelectableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !nextChatGroupIsSelectable(groups, groupID) {
		return nil, ErrGroupNotAllowed
	}
	updated, err := s.realignNextChatManagedKeyGroup(ctx, key, userID, &groupID)
	if err != nil {
		return nil, err
	}
	return s.GetNextChatWorkspaceIdentity(ctx, userID, updated.ID)
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

func (s *APIKeyService) findReusableNextChatManagedKeyByNameAndGroup(ctx context.Context, userID int64, name string, groupID int64) (*APIKey, error) {
	if s == nil || s.apiKeyRepo == nil {
		return nil, fmt.Errorf("api key service is not configured")
	}
	if lister, ok := s.apiKeyRepo.(apiKeyAllByUserIDLister); ok {
		keys, err := lister.ListAllByUserID(ctx, userID, APIKeyListFilters{Status: StatusActive, GroupID: &groupID})
		if err != nil {
			return nil, fmt.Errorf("list managed api keys: %w", err)
		}
		for index := range keys {
			key := &keys[index]
			if key.Name == name && key.IsActive() && !key.IsExpired() && key.GroupID != nil && *key.GroupID == groupID {
				return key, nil
			}
		}
		return nil, nil
	}

	keys, _, err := s.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 100}, APIKeyListFilters{
		Search:  name,
		Status:  StatusActive,
		GroupID: &groupID,
	})
	if err != nil {
		return nil, fmt.Errorf("search managed api keys: %w", err)
	}
	for index := range keys {
		key := &keys[index]
		if key.Name == name && key.IsActive() && !key.IsExpired() && key.GroupID != nil && *key.GroupID == groupID {
			return key, nil
		}
	}
	return nil, nil
}

func nextChatManagedKeyMatchesPurpose(name, purpose string) bool {
	name = strings.TrimSpace(name)
	switch purpose {
	case NextChatSessionPurposeChat:
		return name == NextChatManagedAPIKeyName || name == NextChatManagedChatAPIKeyName || strings.HasPrefix(name, NextChatManagedChatAPIKeyName+"/")
	case NextChatSessionPurposeImage:
		return name == NextChatManagedImageAPIKeyName || strings.HasPrefix(name, NextChatManagedImageAPIKeyName+"/")
	case NextChatSessionPurposeVideo:
		return name == NextChatManagedVideoAPIKeyName || strings.HasPrefix(name, NextChatManagedVideoAPIKeyName+"/")
	default:
		return false
	}
}

func nextChatManagedKeyPurpose(name string) (string, bool) {
	for _, purpose := range []string{
		NextChatSessionPurposeChat,
		NextChatSessionPurposeImage,
		NextChatSessionPurposeVideo,
	} {
		if nextChatManagedKeyMatchesPurpose(name, purpose) {
			return purpose, true
		}
	}
	return "", false
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
	pricing, err := s.ListNextChatDisplayMetadata(ctx, userID)
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
			Models:         []NextChatWorkspaceModel{},
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
	// The workspace's regular model list remains compatible with legacy chat
	// and image behavior. Video is different: it has an independent managed
	// session and per-resolution billing, so it must use the same resolver as
	// mobile jobs rather than infer executability from the generic model list.
	executableVideoModels := s.nextChatExecutableVideoModels(ctx, userID)
	for groupIndex := range out.Groups {
		group := &out.Groups[groupIndex]
		sourceGroup := selectableGroups[groupIndex]
		modelSources, sourceErr := s.nextChatWorkspaceModelSources(ctx, sourceGroup)
		if sourceErr != nil {
			return nil, fmt.Errorf("resolve nextchat media contracts: %w", sourceErr)
		}
		for _, modelSource := range modelSources {
			modelID := strings.TrimSpace(modelSource.ModelID)
			if modelID == "" {
				continue
			}
			normalizedModelID := strings.ToLower(modelID)
			modelGroup := modelSource.Group
			meta := metadata.lookup(group.ID, modelSource.Platform, modelID)
			workspaceModel := buildNextChatWorkspaceModel(modelSource.Platform, modelID, meta)
			upstreamCapabilities := ModelToolCapabilities{}
			if s.pricingService != nil {
				upstreamCapabilities = s.pricingService.GetModelToolCapabilities(modelID)
			}
			workspaceModel.ToolCapabilities = resolveNextChatWorkspaceModelToolCapabilities(
				toolCapabilityEntries,
				modelGroup,
				modelID,
				upstreamCapabilities,
			)
			catalogEntry := findCatalogContractEntry(toolCapabilityEntries, modelGroup, modelID)
			if catalogEntryHasExplicitMediaDeclaration(catalogEntry) {
				if modelSource.ContractFound && len(modelSource.Contract.Modalities) > 0 {
					applyNextChatWorkspaceMediaContract(&workspaceModel, modelSource.Contract)
				} else {
					// An administrator has explicitly reviewed this row. A malformed
					// declaration or an unsupported adapter must fail closed instead
					// of reviving a legacy name-based image profile.
					clearNextChatWorkspaceMediaContract(&workspaceModel)
				}
			} else if modelSource.ContractFound {
				applyNextChatWorkspaceMediaContract(&workspaceModel, modelSource.Contract)
			}
			if resolved, executable := executableVideoModels[group.ID][normalizedModelID]; executable {
				// The mobile resolver selected this exact provider contract after
				// mapping, scheduler, adapter and price checks. Project it as one
				// unit instead of combining its video limits with a first-wins
				// composite source's adapter or version.
				applyNextChatWorkspaceResolvedVideoContract(&workspaceModel, resolved)
			} else if workspaceModel.VideoCapabilities != nil {
				// A missing resolver, a transient catalog/scheduler failure, or a
				// failed executable check can only hide video. It must never
				// degrade a chat or legacy-image model in the same workspace.
				clearNextChatWorkspaceVideoCapability(&workspaceModel)
			}
			if workspaceModel.VideoCapabilities != nil {
				group.VideoAvailable = true
			}
			group.Models = append(group.Models, workspaceModel)
		}
	}

	for i := range out.Groups {
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

// nextChatWorkspaceModelSource retains the concrete platform that made a
// model schedulable. A composite group can route the same public model ID to
// more than one upstream, so the catalog contract must be resolved against
// this platform rather than the composite pseudo-platform.
type nextChatWorkspaceModelSource struct {
	ModelID       string
	Platform      string
	Group         Group
	Contract      GatewayModelContract
	ContractFound bool
}

func (s *ModelCatalogService) nextChatWorkspaceModelSources(
	ctx context.Context,
	group Group,
) ([]nextChatWorkspaceModelSource, error) {
	if s == nil || s.modelResolver == nil {
		return nil, nil
	}
	if group.Platform != PlatformComposite {
		return s.nextChatWorkspaceModelSourcesForPlatform(ctx, group, group.Platform)
	}

	resolver, supportsScheduling := s.modelResolver.(GatewayModelAvailabilityResolver)
	if !supportsScheduling {
		// Older embedded/test resolvers only expose the legacy aggregate view.
		// Preserve it until they implement the scheduler contract; production
		// GatewayService always implements GatewayModelAvailabilityResolver.
		return s.nextChatWorkspaceModelSourcesForPlatform(ctx, group, group.Platform)
	}

	schedulable := resolver.GetSchedulablePlatforms(ctx, &group.ID)
	platforms := CompositeSchedulableProviderPlatforms(schedulable)
	if len(platforms) == 0 {
		// Do not convert a compatibility resolver that only reports the
		// synthetic composite platform into an empty workspace. Real gateway
		// snapshots report concrete account platforms and take the path below.
		return s.nextChatWorkspaceModelSourcesForPlatform(ctx, group, group.Platform)
	}

	seen := make(map[string]struct{})
	result := make([]nextChatWorkspaceModelSource, 0)
	for _, platform := range platforms {
		concreteGroup := group
		concreteGroup.Platform = platform
		sources, err := s.nextChatWorkspaceModelSourcesForPlatform(ctx, concreteGroup, platform)
		if err != nil {
			return nil, err
		}
		for _, source := range sources {
			key := strings.ToLower(strings.TrimSpace(source.ModelID))
			if key == "" {
				continue
			}
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, source)
		}
	}
	return result, nil
}

func (s *ModelCatalogService) nextChatWorkspaceModelSourcesForPlatform(
	ctx context.Context,
	group Group,
	platform string,
) ([]nextChatWorkspaceModelSource, error) {
	availableModels := s.modelResolver.GetAvailableModels(ctx, &group.ID, platform)
	availableModels = filterNextChatWorkspaceModelsForGroup(group, availableModels)
	// The scheduler remains the authority for whether a model can be
	// advertised at all. The catalog only enriches that already-authorized list
	// with media behavior; it must never add a model by itself.
	mediaContracts, err := s.ResolveGatewayModelContracts(ctx, group, availableModels)
	if err != nil {
		return nil, err
	}
	mediaByModel := make(map[string]GatewayModelContract, len(mediaContracts))
	for _, contract := range mediaContracts {
		if key := strings.ToLower(strings.TrimSpace(contract.ID)); key != "" {
			mediaByModel[key] = contract
		}
	}

	seen := make(map[string]struct{}, len(availableModels))
	result := make([]nextChatWorkspaceModelSource, 0, len(availableModels))
	for _, modelID := range availableModels {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		key := strings.ToLower(modelID)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		contract, found := mediaByModel[key]
		result = append(result, nextChatWorkspaceModelSource{
			ModelID: modelID, Platform: platform, Group: group,
			Contract: contract, ContractFound: found,
		})
	}
	return result, nil
}

// nextChatExecutableVideoModels intentionally delegates to the mobile-video
// resolver instead of reproducing its group, account, mapping, capability, and
// price checks. GatewayService supplies the richer resolver in production; a
// list-only implementation fails closed for video while preserving existing
// chat/image workspace rows.
func (s *ModelCatalogService) nextChatExecutableVideoModels(
	ctx context.Context,
	userID int64,
) map[int64]map[string]MobileVideoResolvedModel {
	if s == nil || s.apiKeyService == nil || s.repo == nil || userID <= 0 {
		return nil
	}
	models, ok := s.modelResolver.(GatewayModelAvailabilityResolver)
	if !ok {
		return nil
	}

	bootstrap, err := NewCatalogMobileVideoAvailabilityResolver(s.apiKeyService, s, models).Bootstrap(ctx, userID)
	if err != nil {
		return nil
	}

	resolved := make(map[int64]map[string]MobileVideoResolvedModel, len(bootstrap.Groups))
	for _, group := range bootstrap.Groups {
		if len(group.Models) == 0 {
			continue
		}
		byModel := make(map[string]MobileVideoResolvedModel, len(group.Models))
		for _, model := range group.Models {
			key := strings.ToLower(strings.TrimSpace(model.Model))
			if key != "" {
				byModel[key] = model
			}
		}
		if len(byModel) > 0 {
			resolved[group.ID] = byModel
		}
	}
	return resolved
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

func buildNextChatWorkspaceModelMetadata(rows []NextChatDisplayModel) nextChatWorkspaceModelMetadata {
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
	if capability, ok := ResolveAuditedLegacyWorkspaceImageCapability(platform, modelID); ok {
		model.ImageCapabilities = modelImageCapabilitiesFromImageStudio(capability)
		// Only exact profiles reviewed before the media catalog are retained as a
		// temporary compatibility declaration. New mappings require an explicit
		// catalog contract and must never gain image capability from their name.
		model.Modalities = []string{"image"}
		model.Adapter = nextChatLegacyImageAdapter(platform, capability)
		model.CapabilityVersion = capability.Revision
	}
	return model
}

func applyNextChatWorkspaceMediaContract(model *NextChatWorkspaceModel, contract GatewayModelContract) {
	if model == nil || len(contract.Modalities) == 0 {
		return
	}
	// An explicit declaration takes precedence over the compatibility profile.
	// This lets an administrator retire an old inferred capability without
	// needing a separate client release.
	model.Modalities = cloneCatalogStrings(contract.Modalities)
	model.Adapter = strings.TrimSpace(contract.Adapter)
	model.CapabilityVersion = strings.TrimSpace(contract.CapabilityVersion)
	model.ImageCapabilities = nextChatImageCapabilitiesFromCatalog(contract.ImageCapabilities)
	model.VideoCapabilities = cloneNextChatVideoCapabilities(contract.VideoCapabilities)
}

func applyNextChatWorkspaceResolvedVideoContract(model *NextChatWorkspaceModel, resolved MobileVideoResolvedModel) {
	if model == nil {
		return
	}
	contract, valid := GatewayModelContractFromMobileVideo(resolved)
	if !valid {
		clearNextChatWorkspaceVideoCapability(model)
		return
	}

	// A top-level adapter/version cannot describe two providers. Preserve an
	// image declaration only when it is already tied to this exact selected
	// video contract; otherwise fail closed for image rather than advertise a
	// hybrid model that neither upstream can execute.
	preserveImage := model.ImageCapabilities != nil &&
		strings.EqualFold(strings.TrimSpace(model.Adapter), contract.Adapter) &&
		strings.TrimSpace(model.CapabilityVersion) == contract.CapabilityVersion
	if preserveImage {
		modalities := make([]string, 0, len(model.Modalities)+1)
		for _, modality := range model.Modalities {
			if !strings.EqualFold(strings.TrimSpace(modality), "video") {
				modalities = append(modalities, modality)
			}
		}
		modalities = append(modalities, "video")
		model.Modalities = modalities
	} else {
		model.Modalities = []string{"video"}
		model.ImageCapabilities = nil
	}
	model.Platform = contract.Platform
	model.Adapter = contract.Adapter
	model.CapabilityVersion = contract.CapabilityVersion
	model.VideoCapabilities = cloneNextChatVideoCapabilities(contract.VideoCapabilities)
}

// catalogEntryHasExplicitMediaDeclaration distinguishes a legacy NULL row from
// an administrator-reviewed row. Invalid JSON is deliberately explicit here:
// it must not fall back to a static media profile.
func catalogEntryHasExplicitMediaDeclaration(entry *SiteModelCatalogEntry) bool {
	if entry == nil {
		return false
	}
	declaration, err := ParseCatalogMediaCapabilities(entry.MediaCapabilities)
	return declaration != nil || err != nil
}

func clearNextChatWorkspaceMediaContract(model *NextChatWorkspaceModel) {
	if model == nil {
		return
	}
	model.Modalities = nil
	model.Adapter = ""
	model.CapabilityVersion = ""
	model.ImageCapabilities = nil
	model.VideoCapabilities = nil
}

// clearNextChatWorkspaceVideoCapability removes only the video projection.
// A catalog row can intentionally carry chat and image alongside video, so a
// failed video preflight must not erase the remaining declared modes.
func clearNextChatWorkspaceVideoCapability(model *NextChatWorkspaceModel) {
	if model == nil {
		return
	}
	modalities := make([]string, 0, len(model.Modalities))
	for _, modality := range model.Modalities {
		if !strings.EqualFold(strings.TrimSpace(modality), "video") {
			modalities = append(modalities, modality)
		}
	}
	model.Modalities = modalities
	model.VideoCapabilities = nil
	if len(model.Modalities) == 0 {
		model.Adapter = ""
		model.CapabilityVersion = ""
	}
}

func nextChatImageCapabilitiesFromCatalog(capability *ModelImageCapabilities) *ModelImageCapabilities {
	if capability == nil {
		return nil
	}
	image := capability.Clone()
	return &image
}

func modelImageCapabilitiesFromImageStudio(capability ImageStudioModelCapabilities) *ModelImageCapabilities {
	image := ModelImageCapabilities{
		Operations:         cloneCatalogStrings(capability.Operations),
		SizingKind:         strings.TrimSpace(capability.SizingKind),
		SupportedSizes:     cloneCatalogStrings(capability.SupportedSizes),
		SupportedRatios:    cloneCatalogStrings(capability.SupportedAspectRatios),
		SupportedFormats:   cloneCatalogStrings(capability.SupportedOutputFormats),
		MinDimension:       capability.MinDimension,
		MaxDimension:       capability.MaxDimension,
		DimensionStep:      capability.DimensionStep,
		MaxAspectRatio:     capability.MaxAspectRatio,
		MaxReferenceImages: capability.MaxReferenceImages,
	}
	return &image
}

func cloneNextChatVideoCapabilities(capability *MobileVideoCapabilities) *MobileVideoCapabilities {
	return cloneGatewayVideoCapabilities(capability)
}

func nextChatLegacyImageAdapter(platform string, capability ImageStudioModelCapabilities) string {
	if providerID := strings.TrimSpace(capability.ProviderID); providerID != "" {
		return providerID
	}
	switch normalizeCatalogContractPlatform(platform) {
	case PlatformGemini:
		return "gemini_images"
	case PlatformGrok:
		return "grok_images"
	default:
		return "openai_images"
	}
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
