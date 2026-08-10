package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

	"github.com/redis/go-redis/v9"
)

// ForumSSOService makes this platform the OAuth2 identity provider, wallet and
// VIP authority for the NodeBB community forum.
//
// TOKEN DESIGN
// ------------
// The SSO access token is an opaque random string kept in Redis, NOT a platform
// JWT. The forum persists this token per user (user-sync.js writes it to
// `sub2api:access_token`) and replays it for months. Handing it a platform JWT
// would give the forum a credential that unlocks the entire panel API, and it
// would silently expire with no refresh path. An opaque token lets us scope it
// to /api/v1/sso/* and revoke it server-side.
var (
	ErrForumSSODisabled       = infraerrors.NotFound("FORUM_SSO_DISABLED", "forum sso is not enabled")
	ErrForumSSOUnknownClient  = infraerrors.Unauthorized("FORUM_SSO_UNKNOWN_CLIENT", "unknown client_id")
	ErrForumSSOBadRedirectURI = infraerrors.BadRequest("FORUM_SSO_BAD_REDIRECT_URI", "redirect_uri is not allowlisted")
	ErrForumSSOBadCode        = infraerrors.Unauthorized("FORUM_SSO_BAD_CODE", "authorization code is invalid or expired")
	ErrForumSSOBadToken       = infraerrors.Unauthorized("FORUM_SSO_BAD_TOKEN", "access token is invalid or expired")
	ErrForumSSOBadClientAuth  = infraerrors.Unauthorized("FORUM_SSO_BAD_CLIENT_AUTH", "client authentication failed")
	ErrForumSSOStoreDown      = infraerrors.InternalServer("FORUM_SSO_STORE_UNAVAILABLE", "forum sso store is unavailable")
)

const (
	forumSSOCodePrefix  = "forum_sso:code:"
	forumSSOTokenPrefix = "forum_sso:token:"
	forumSSOUserPrefix  = "forum_sso:user_tokens:"

	forumSSODefaultCodeTTL   = 60 * time.Second
	forumSSODefaultTokenTTL  = 30 * 24 * time.Hour
	forumSSOMaxTokensPerUser = 5
)

type ForumSSOService struct {
	cfg            *config.Config
	redis          *redis.Client
	userService    *UserService
	playService    *PlayService
	settingService *SettingService
	ledger         *BalanceLedgerService
	now            func() time.Time
}

func NewForumSSOService(
	cfg *config.Config,
	redisClient *redis.Client,
	userService *UserService,
	playService *PlayService,
	settingService *SettingService,
	ledger *BalanceLedgerService,
) *ForumSSOService {
	return &ForumSSOService{
		cfg:            cfg,
		redis:          redisClient,
		userService:    userService,
		playService:    playService,
		settingService: settingService,
		ledger:         ledger,
		now:            time.Now,
	}
}

func (s *ForumSSOService) conf() config.ForumSSOConfig {
	if s == nil || s.cfg == nil {
		return config.ForumSSOConfig{}
	}
	return s.cfg.ForumSSO
}

// Enabled reports whether the integration is configured well enough to serve.
// A missing client secret is treated as disabled rather than as an open client,
// so a half-applied deployment cannot accidentally accept any secret.
func (s *ForumSSOService) Enabled() bool {
	c := s.conf()
	return c.Enabled && c.ClientID != "" && c.ClientSecret != "" && s.redis != nil
}

// LoginPagePath is the frontend route that resumes an authorize request when the
// browser arrives without a platform session.
func (s *ForumSSOService) LoginPagePath() string {
	if path := strings.TrimSpace(s.conf().LoginPagePath); path != "" {
		return path
	}
	return "/login"
}

func (s *ForumSSOService) codeTTL() time.Duration {
	if ttl := s.conf().AuthCodeTTLSeconds; ttl > 0 {
		return time.Duration(ttl) * time.Second
	}
	return forumSSODefaultCodeTTL
}

func (s *ForumSSOService) tokenTTL() time.Duration {
	if ttl := s.conf().AccessTokenTTLSeconds; ttl > 0 {
		return time.Duration(ttl) * time.Second
	}
	return forumSSODefaultTokenTTL
}

// VerifyClientID compares in constant time so a wrong client_id cannot be
// discovered by timing the response.
func (s *ForumSSOService) VerifyClientID(clientID string) error {
	if !s.Enabled() {
		return ErrForumSSODisabled
	}
	expected := s.conf().ClientID
	if subtle.ConstantTimeCompare([]byte(clientID), []byte(expected)) != 1 {
		return ErrForumSSOUnknownClient
	}
	return nil
}

// VerifyClientCredentials authenticates the confidential client at the token
// endpoint. Both values are compared in constant time.
func (s *ForumSSOService) VerifyClientCredentials(clientID, clientSecret string) error {
	if !s.Enabled() {
		return ErrForumSSODisabled
	}
	c := s.conf()
	idOK := subtle.ConstantTimeCompare([]byte(clientID), []byte(c.ClientID)) == 1
	secretOK := subtle.ConstantTimeCompare([]byte(clientSecret), []byte(c.ClientSecret)) == 1
	// Evaluate both before branching so the reply time does not reveal which
	// half was wrong.
	if !idOK || !secretOK {
		return ErrForumSSOBadClientAuth
	}
	return nil
}

// AllowedRedirectURIs returns the parsed exact-match allowlist.
func (s *ForumSSOService) AllowedRedirectURIs() []string {
	raw := strings.Split(s.conf().RedirectURLs, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// VerifyRedirectURI enforces exact string equality against the allowlist.
//
// Deliberately not a prefix or host comparison: prefix matching on a redirect
// URI is a classic open-redirect that leaks authorization codes, since
// "https://forum/cb" would also admit "https://forum/cb.attacker.test".
func (s *ForumSSOService) VerifyRedirectURI(redirectURI string) error {
	if redirectURI == "" {
		return ErrForumSSOBadRedirectURI
	}
	for _, allowed := range s.AllowedRedirectURIs() {
		if subtle.ConstantTimeCompare([]byte(redirectURI), []byte(allowed)) == 1 {
			return nil
		}
	}
	return ErrForumSSOBadRedirectURI
}

func forumSSORandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// forumSSOCodeRecord is what an authorization code redeems into. redirect_uri is
// stored so the token endpoint can enforce RFC 6749 §4.1.3: the redirect_uri
// presented at redemption must equal the one the code was issued for.
type forumSSOCodeRecord struct {
	UserID      int64  `json:"user_id"`
	ClientID    string `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
	Scope       string `json:"scope"`
	IssuedAt    int64  `json:"issued_at"`
}

// IssueAuthorizationCode mints a single-use code bound to the user, client and
// redirect_uri.
func (s *ForumSSOService) IssueAuthorizationCode(ctx context.Context, userID int64, clientID, redirectURI, scope string) (string, error) {
	if !s.Enabled() {
		return "", ErrForumSSODisabled
	}
	code, err := forumSSORandomToken()
	if err != nil {
		return "", ErrForumSSOStoreDown
	}
	record := forumSSOCodeRecord{
		UserID:      userID,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		Scope:       scope,
		IssuedAt:    s.now().Unix(),
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", ErrForumSSOStoreDown
	}
	if err := s.redis.Set(ctx, forumSSOCodePrefix+code, encoded, s.codeTTL()).Err(); err != nil {
		return "", ErrForumSSOStoreDown
	}
	return code, nil
}

// RedeemAuthorizationCode consumes a code exactly once and returns the bound
// record. GetDel makes redemption atomic, so two concurrent token requests with
// the same code cannot both succeed.
func (s *ForumSSOService) RedeemAuthorizationCode(ctx context.Context, code, clientID, redirectURI string) (*forumSSOCodeRecord, error) {
	if !s.Enabled() {
		return nil, ErrForumSSODisabled
	}
	if code == "" {
		return nil, ErrForumSSOBadCode
	}

	raw, err := s.redis.GetDel(ctx, forumSSOCodePrefix+code).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrForumSSOBadCode
		}
		return nil, ErrForumSSOStoreDown
	}

	var record forumSSOCodeRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, ErrForumSSOBadCode
	}
	if record.ClientID != clientID {
		return nil, ErrForumSSOBadCode
	}
	// RFC 6749 §4.1.3: the redirect_uri must match the authorization request.
	if record.RedirectURI != redirectURI {
		return nil, ErrForumSSOBadCode
	}
	return &record, nil
}

// IssueAccessToken mints an opaque SSO token for the user and indexes it so all
// of a user's tokens can be revoked together.
func (s *ForumSSOService) IssueAccessToken(ctx context.Context, userID int64, scope string) (string, int, error) {
	if !s.Enabled() {
		return "", 0, ErrForumSSODisabled
	}
	token, err := forumSSORandomToken()
	if err != nil {
		return "", 0, ErrForumSSOStoreDown
	}
	ttl := s.tokenTTL()
	payload, err := json.Marshal(map[string]any{
		"user_id":   userID,
		"scope":     scope,
		"issued_at": s.now().Unix(),
	})
	if err != nil {
		return "", 0, ErrForumSSOStoreDown
	}

	if err := s.redis.Set(ctx, forumSSOTokenPrefix+token, payload, ttl).Err(); err != nil {
		return "", 0, ErrForumSSOStoreDown
	}

	// Bounded per-user token index. Without the cap, a user who logs in from the
	// forum repeatedly would grow this set without limit.
	indexKey := forumSSOUserPrefix + strconv.FormatInt(userID, 10)
	score := float64(s.now().UnixNano())
	if err := s.redis.ZAdd(ctx, indexKey, redis.Z{Score: score, Member: token}).Err(); err == nil {
		s.redis.Expire(ctx, indexKey, ttl)
		if stale, err := s.redis.ZRevRange(ctx, indexKey, forumSSOMaxTokensPerUser, -1).Result(); err == nil && len(stale) > 0 {
			keys := make([]string, 0, len(stale))
			members := make([]any, 0, len(stale))
			for _, old := range stale {
				keys = append(keys, forumSSOTokenPrefix+old)
				members = append(members, old)
			}
			s.redis.Del(ctx, keys...)
			s.redis.ZRem(ctx, indexKey, members...)
		}
	}

	return token, int(ttl.Seconds()), nil
}

// ResolveAccessToken maps an opaque SSO token back to a user ID.
func (s *ForumSSOService) ResolveAccessToken(ctx context.Context, token string) (int64, error) {
	if !s.Enabled() {
		return 0, ErrForumSSODisabled
	}
	if token == "" {
		return 0, ErrForumSSOBadToken
	}
	raw, err := s.redis.Get(ctx, forumSSOTokenPrefix+token).Bytes()
	if err != nil {
		if err == redis.Nil {
			return 0, ErrForumSSOBadToken
		}
		return 0, ErrForumSSOStoreDown
	}
	var payload struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.UserID <= 0 {
		return 0, ErrForumSSOBadToken
	}
	return payload.UserID, nil
}

// RevokeUserTokens drops every SSO token for a user. Called when an account is
// disabled so the forum session cannot outlive platform access.
func (s *ForumSSOService) RevokeUserTokens(ctx context.Context, userID int64) error {
	if s == nil || s.redis == nil {
		return nil
	}
	indexKey := forumSSOUserPrefix + strconv.FormatInt(userID, 10)
	tokens, err := s.redis.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(tokens)+1)
	for _, token := range tokens {
		keys = append(keys, forumSSOTokenPrefix+token)
	}
	keys = append(keys, indexKey)
	return s.redis.Del(ctx, keys...).Err()
}

// ForumUserInfo is the OAuth2 userinfo document consumed by the forum plugin's
// user-sync.upsertUser(). Field names are dictated by that consumer, so the
// platform's usual {code,message,data} envelope must NOT wrap it.
type ForumUserInfo struct {
	ID               int64    `json:"id"`
	Email            string   `json:"email"`
	EmailVerified    bool     `json:"email_verified"`
	Username         string   `json:"username"`
	Balance          string   `json:"balance"`
	VIPTier          int      `json:"vip_tier"`
	VIPLabel         string   `json:"vip_label"`
	RechargeBonusPct float64  `json:"recharge_bonus_pct"`
	Role             string   `json:"role"`
	Groups           []string `json:"groups"`
	Language         string   `json:"language"`
}

// BuildUserInfo assembles the userinfo document for a platform user.
func (s *ForumSSOService) BuildUserInfo(ctx context.Context, userID int64) (*ForumUserInfo, error) {
	user, err := s.userService.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrForumSSOBadToken
	}
	// A disabled or deleted account must not be able to keep opening forum
	// sessions.
	if !user.IsActive() || user.DeletedAt != nil {
		return nil, infraerrors.Forbidden("FORUM_SSO_USER_INACTIVE", "user is not active")
	}

	vip := s.resolveVIP(ctx, user)

	// The forum derives NodeBB group membership from this list. vip-<tier>
	// matches the plugin's own expectation (webhook-handlers.js builds the same
	// name), and registered-users is NodeBB's baseline group.
	groups := []string{"registered-users"}
	if vip.Tier > 0 {
		groups = append(groups, fmt.Sprintf("vip-%d", vip.Tier))
	}
	if user.IsAdmin() {
		groups = append(groups, "administrators")
	}

	username := strings.TrimSpace(user.Username)
	if username == "" {
		username = fmt.Sprintf("sub2_%d", user.ID)
	}

	return &ForumUserInfo{
		ID:    user.ID,
		Email: user.Email,
		// The forum refuses to link an SSO identity onto an existing forum
		// account by email unless the address is verified. Platform accounts are
		// created against a verified address, and a false value here would break
		// legitimate linking, so this asserts the platform's own guarantee.
		EmailVerified:    true,
		Username:         username,
		Balance:          formatForumMoney(user.Balance),
		VIPTier:          vip.Tier,
		VIPLabel:         vip.Label,
		RechargeBonusPct: vip.RechargeBonusPct,
		Role:             user.Role,
		Groups:           groups,
		Language:         "zh-CN",
	}, nil
}

// resolveVIP computes tier from the same source the admin panel uses: the sum of
// verified membership order contributions, not users.total_recharged. Using the
// latter would show a different tier in the forum than in the panel.
func (s *ForumSSOService) resolveVIP(ctx context.Context, user *User) PlayVIPStatus {
	tiers := []PlayVIPTier(nil)
	if s.settingService != nil {
		tiers = s.settingService.GetPlayRuntime(ctx).VIPTiers
	}

	paid := user.MembershipPaidAmount
	if s.playService != nil {
		if total, err := s.playService.MembershipPaidTotal(ctx, user.ID); err == nil {
			paid = total
		}
	}
	return GetVIPTier(paid, tiers)
}

// formatForumMoney renders money as a fixed-scale string. Both the userinfo
// document and every signed webhook payload use this: the canonical signature
// stringifies values, and a JSON number 10.00 would arrive as "10" on the
// NodeBB side and fail verification.
func formatForumMoney(amount float64) string {
	return strconv.FormatFloat(amount, 'f', 2, 64)
}
