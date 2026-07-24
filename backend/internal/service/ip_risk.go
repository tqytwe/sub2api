package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelSevere   RiskLevel = "severe"
	RiskLevelCritical RiskLevel = "critical"
)

type EvidenceConfidence string

const (
	EvidenceConfidenceExact    EvidenceConfidence = "exact"
	EvidenceConfidenceInferred EvidenceConfidence = "inferred"
)

type RiskSignalFamily string

const (
	RiskSignalFamilyRegistration RiskSignalFamily = "registration"
	RiskSignalFamilyIdentity     RiskSignalFamily = "identity"
	RiskSignalFamilyBehavior     RiskSignalFamily = "behavior"
	RiskSignalFamilySignupCode   RiskSignalFamily = "signup_code"
	RiskSignalFamilyTrust        RiskSignalFamily = "trust"
)

type RiskSignalCode string

const (
	RiskSignalRegistration10m  RiskSignalCode = "registration_10m"
	RiskSignalRegistration1h   RiskSignalCode = "registration_1h"
	RiskSignalRegistration24h  RiskSignalCode = "registration_24h"
	RiskSignalSharedUA3        RiskSignalCode = "shared_ua_3"
	RiskSignalSharedUA5        RiskSignalCode = "shared_ua_5"
	RiskSignalEmailPattern     RiskSignalCode = "email_pattern"
	RiskSignalSharedAPIIP      RiskSignalCode = "shared_api_ip"
	RiskSignalRapidKeyOrGift   RiskSignalCode = "rapid_key_or_gift"
	RiskSignalSharedSignupCode RiskSignalCode = "shared_signup_code"
	RiskSignalTrustedAccount   RiskSignalCode = "trusted_account"
)

type IPRiskSignal struct {
	Code   RiskSignalCode   `json:"code"`
	Family RiskSignalFamily `json:"family"`
	Score  int              `json:"score"`
	Count  int              `json:"count,omitempty"`
}

type IPRiskConfig struct {
	Registration10mThreshold  int
	Registration10mScore      int
	Registration1hThreshold   int
	Registration1hScore       int
	Registration24hThreshold  int
	Registration24hScore      int
	SharedUA3Threshold        int
	SharedUA3Score            int
	SharedUA5Threshold        int
	SharedUA5Score            int
	EmailPatternThreshold     int
	EmailPatternScore         int
	SharedAPIIPThreshold      int
	SharedAPIIPScore          int
	RapidBehaviorThreshold    int
	RapidBehaviorScore        int
	SharedSignupCodeThreshold int
	SharedSignupCodeScore     int
	TrustedAccountScore       int
	AutoBlockScore            int
	AutoBlockMinRegistrations int
	AutoBlockDuration         time.Duration
}

func DefaultIPRiskConfig() IPRiskConfig {
	return IPRiskConfig{
		Registration10mThreshold:  3,
		Registration10mScore:      25,
		Registration1hThreshold:   5,
		Registration1hScore:       35,
		Registration24hThreshold:  10,
		Registration24hScore:      45,
		SharedUA3Threshold:        3,
		SharedUA3Score:            15,
		SharedUA5Threshold:        5,
		SharedUA5Score:            20,
		EmailPatternThreshold:     3,
		EmailPatternScore:         15,
		SharedAPIIPThreshold:      3,
		SharedAPIIPScore:          25,
		RapidBehaviorThreshold:    2,
		RapidBehaviorScore:        15,
		SharedSignupCodeThreshold: 3,
		SharedSignupCodeScore:     10,
		TrustedAccountScore:       -15,
		AutoBlockScore:            90,
		AutoBlockMinRegistrations: 5,
		AutoBlockDuration:         30 * time.Minute,
	}
}

type IPRiskEvidence struct {
	PrimaryIP                  string `json:"primary_ip"`
	PrimaryNetwork             string `json:"primary_network"`
	PrimaryIPRegistrationCount int    `json:"primary_ip_registration_count"`
	RegistrationCount10m       int    `json:"registration_count_10m"`
	RegistrationCount1h        int    `json:"registration_count_1h"`
	RegistrationCount24h       int    `json:"registration_count_24h"`
	ExactRegistrationCount     int    `json:"exact_registration_count"`
	MaxSharedUACount           int    `json:"max_shared_ua_count"`
	EmailPatternAccountCount   int    `json:"email_pattern_account_count"`
	SharedAPIIPUserCount       int    `json:"shared_api_ip_user_count"`
	RapidKeyOrGiftUserCount    int    `json:"rapid_key_or_gift_user_count"`
	SharedSignupCodeCount      int    `json:"shared_signup_code_count"`
	TrustedAccountCount        int    `json:"trusted_account_count"`
	AllKeyEvidenceExact        bool   `json:"all_key_evidence_exact"`
	Allowlisted                bool   `json:"allowlisted"`
	KnownSharedNetwork         bool   `json:"known_shared_network"`
}

type IPRiskAssessment struct {
	Score                  int            `json:"score"`
	Level                  RiskLevel      `json:"level"`
	Signals                []IPRiskSignal `json:"signals"`
	PositiveSignalFamilies int            `json:"positive_signal_families"`
	AutoBlockEligible      bool           `json:"auto_block_eligible"`
	AutoBlockTarget        string         `json:"auto_block_target,omitempty"`
	AutoBlockDuration      time.Duration  `json:"auto_block_duration,omitempty"`
}

func CalculateIPRiskAssessment(config IPRiskConfig, evidence IPRiskEvidence) IPRiskAssessment {
	signals := make([]IPRiskSignal, 0, 8)

	switch {
	case evidence.RegistrationCount24h >= config.Registration24hThreshold:
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalRegistration24h, Family: RiskSignalFamilyRegistration,
			Score: config.Registration24hScore, Count: evidence.RegistrationCount24h,
		})
	case evidence.RegistrationCount1h >= config.Registration1hThreshold:
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalRegistration1h, Family: RiskSignalFamilyRegistration,
			Score: config.Registration1hScore, Count: evidence.RegistrationCount1h,
		})
	case evidence.RegistrationCount10m >= config.Registration10mThreshold:
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalRegistration10m, Family: RiskSignalFamilyRegistration,
			Score: config.Registration10mScore, Count: evidence.RegistrationCount10m,
		})
	}

	switch {
	case evidence.MaxSharedUACount >= config.SharedUA5Threshold:
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalSharedUA5, Family: RiskSignalFamilyIdentity,
			Score: config.SharedUA5Score, Count: evidence.MaxSharedUACount,
		})
	case evidence.MaxSharedUACount >= config.SharedUA3Threshold:
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalSharedUA3, Family: RiskSignalFamilyIdentity,
			Score: config.SharedUA3Score, Count: evidence.MaxSharedUACount,
		})
	}

	if evidence.EmailPatternAccountCount >= config.EmailPatternThreshold {
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalEmailPattern, Family: RiskSignalFamilyIdentity,
			Score: config.EmailPatternScore, Count: evidence.EmailPatternAccountCount,
		})
	}
	if evidence.SharedAPIIPUserCount >= config.SharedAPIIPThreshold {
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalSharedAPIIP, Family: RiskSignalFamilyBehavior,
			Score: config.SharedAPIIPScore, Count: evidence.SharedAPIIPUserCount,
		})
	}
	if evidence.RapidKeyOrGiftUserCount >= config.RapidBehaviorThreshold {
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalRapidKeyOrGift, Family: RiskSignalFamilyBehavior,
			Score: config.RapidBehaviorScore, Count: evidence.RapidKeyOrGiftUserCount,
		})
	}
	if evidence.SharedSignupCodeCount >= config.SharedSignupCodeThreshold {
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalSharedSignupCode, Family: RiskSignalFamilySignupCode,
			Score: config.SharedSignupCodeScore, Count: evidence.SharedSignupCodeCount,
		})
	}
	if evidence.TrustedAccountCount > 0 {
		signals = append(signals, IPRiskSignal{
			Code: RiskSignalTrustedAccount, Family: RiskSignalFamilyTrust,
			Score: config.TrustedAccountScore, Count: evidence.TrustedAccountCount,
		})
	}

	score := 0
	positiveFamilies := make(map[RiskSignalFamily]struct{})
	hasRegistrationSignal := false
	for _, signal := range signals {
		score += signal.Score
		if signal.Score > 0 {
			positiveFamilies[signal.Family] = struct{}{}
		}
		if signal.Family == RiskSignalFamilyRegistration && signal.Score > 0 {
			hasRegistrationSignal = true
		}
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	assessment := IPRiskAssessment{
		Score:                  score,
		Level:                  riskLevelForScore(score),
		Signals:                signals,
		PositiveSignalFamilies: len(positiveFamilies),
		AutoBlockDuration:      config.AutoBlockDuration,
	}
	assessment.AutoBlockEligible =
		score >= config.AutoBlockScore &&
			evidence.PrimaryIPRegistrationCount >= config.AutoBlockMinRegistrations &&
			len(positiveFamilies) >= 2 &&
			hasRegistrationSignal &&
			evidence.AllKeyEvidenceExact &&
			!evidence.Allowlisted &&
			!evidence.KnownSharedNetwork
	if assessment.AutoBlockEligible {
		if address, err := NormalizeIPRiskAddress(evidence.PrimaryIP); err == nil {
			if address.Family == 6 {
				assessment.AutoBlockTarget = address.Exact + "/128"
			} else {
				assessment.AutoBlockTarget = address.Exact + "/32"
			}
		}
	}
	return assessment
}

func riskLevelForScore(score int) RiskLevel {
	switch {
	case score >= 90:
		return RiskLevelCritical
	case score >= 80:
		return RiskLevelSevere
	case score >= 60:
		return RiskLevelHigh
	case score >= 40:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

type IPRiskAddress struct {
	Exact   string
	Network string
	Family  int
}

func NormalizeIPRiskAddress(raw string) (IPRiskAddress, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return IPRiskAddress{}, errors.New("empty IP address")
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")

	address, err := netip.ParseAddr(value)
	if err != nil {
		return IPRiskAddress{}, err
	}
	address = address.Unmap()
	bits := 32
	family := 4
	if address.Is6() {
		bits = 64
		family = 6
	}
	return IPRiskAddress{
		Exact:   address.String(),
		Network: netip.PrefixFrom(address, bits).Masked().String(),
		Family:  family,
	}, nil
}

type IPRiskRequestMetadata struct {
	ClientIP   string
	UserAgent  string
	RequestID  string
	OccurredAt time.Time
}

type ipRiskRequestMetadataContextKey struct{}

func WithIPRiskRequestMetadata(ctx context.Context, metadata IPRiskRequestMetadata) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ipRiskRequestMetadataContextKey{}, metadata)
}

func IPRiskRequestMetadataFromContext(ctx context.Context) IPRiskRequestMetadata {
	if ctx == nil {
		return IPRiskRequestMetadata{}
	}
	value, _ := ctx.Value(ipRiskRequestMetadataContextKey{}).(IPRiskRequestMetadata)
	return value
}

var (
	ipRiskWhitespacePattern  = regexp.MustCompile(`\s+`)
	ipRiskVersionPattern     = regexp.MustCompile(`\d+(?:[._-]\d+)*`)
	ipRiskEmailDigitsPattern = regexp.MustCompile(`\d+`)
)

type IPRiskUserAgentFingerprint struct {
	Summary string
	Digest  string
}

type IPRiskEmailPattern struct {
	Digest       string
	TemplateLike bool
}

type IPRiskHasher struct {
	key []byte
}

func NewIPRiskHasher(key []byte) *IPRiskHasher {
	return &IPRiskHasher{key: append([]byte(nil), key...)}
}

func (h *IPRiskHasher) digest(domain, value string) string {
	mac := hmac.New(sha256.New, h.key)
	_, _ = mac.Write([]byte("ip-risk:" + domain + "\x00" + value))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *IPRiskHasher) UserAgent(raw string) IPRiskUserAgentFingerprint {
	summary := strings.ToLower(strings.TrimSpace(raw))
	summary = ipRiskWhitespacePattern.ReplaceAllString(summary, " ")
	summary = ipRiskVersionPattern.ReplaceAllString(summary, "{v}")
	summary = truncateIPRiskUTF8(summary, 160)
	if summary == "" {
		return IPRiskUserAgentFingerprint{}
	}
	return IPRiskUserAgentFingerprint{
		Summary: summary,
		Digest:  h.digest("ua", summary),
	}
}

func (h *IPRiskHasher) EmailPattern(email string) IPRiskEmailPattern {
	email = strings.ToLower(strings.TrimSpace(email))
	local := email
	if index := strings.LastIndexByte(email, '@'); index >= 0 {
		local = email[:index]
	}
	templateLike := ipRiskEmailDigitsPattern.MatchString(local)
	pattern := ipRiskEmailDigitsPattern.ReplaceAllString(local, "{n}")
	return IPRiskEmailPattern{
		Digest:       h.digest("email-pattern", pattern),
		TemplateLike: templateLike,
	}
}

func (h *IPRiskHasher) OpaqueCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return h.digest("opaque-code", value)
}

func truncateIPRiskUTF8(value string, maxBytes int) string {
	value = strings.ToValidUTF8(value, "")
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end]
}
