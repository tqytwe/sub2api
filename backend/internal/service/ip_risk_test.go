package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestNormalizeIPRiskAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		wantExact   string
		wantNetwork string
		wantFamily  int
	}{
		{
			name:        "ipv4 with port",
			raw:         "203.0.113.8:443",
			wantExact:   "203.0.113.8",
			wantNetwork: "203.0.113.8/32",
			wantFamily:  4,
		},
		{
			name:        "ipv4 mapped ipv6",
			raw:         "::ffff:198.51.100.42",
			wantExact:   "198.51.100.42",
			wantNetwork: "198.51.100.42/32",
			wantFamily:  4,
		},
		{
			name:        "ipv6 aggregates to slash 64",
			raw:         "[2001:db8:7a4::19]:8443",
			wantExact:   "2001:db8:7a4::19",
			wantNetwork: "2001:db8:7a4::/64",
			wantFamily:  6,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeIPRiskAddress(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.wantExact, got.Exact)
			require.Equal(t, tt.wantNetwork, got.Network)
			require.Equal(t, tt.wantFamily, got.Family)
		})
	}

	_, err := NormalizeIPRiskAddress("not-an-ip")
	require.Error(t, err)
}

func TestCalculateIPRiskAssessmentUsesHighestRegistrationTierOnly(t *testing.T) {
	t.Parallel()

	config := DefaultIPRiskConfig()
	assessment := CalculateIPRiskAssessment(config, IPRiskEvidence{
		RegistrationCount10m:   3,
		RegistrationCount1h:    5,
		RegistrationCount24h:   10,
		ExactRegistrationCount: 10,
	})

	require.Equal(t, 45, assessment.Score)
	require.Equal(t, RiskLevelMedium, assessment.Level)
	require.Equal(t, []RiskSignalCode{RiskSignalRegistration24h}, signalCodes(assessment.Signals))
}

func TestCalculateIPRiskAssessmentUsesHighestUATierOnly(t *testing.T) {
	t.Parallel()

	assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), IPRiskEvidence{
		MaxSharedUACount: 5,
	})

	require.Equal(t, 20, assessment.Score)
	require.Equal(t, []RiskSignalCode{RiskSignalSharedUA5}, signalCodes(assessment.Signals))
}

func TestCalculateIPRiskAssessmentSignalThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		evidence  IPRiskEvidence
		wantScore int
		wantCode  RiskSignalCode
	}{
		{
			name:      "registration ten minutes",
			evidence:  IPRiskEvidence{RegistrationCount10m: 3},
			wantScore: 25,
			wantCode:  RiskSignalRegistration10m,
		},
		{
			name:      "registration one hour",
			evidence:  IPRiskEvidence{RegistrationCount1h: 5},
			wantScore: 35,
			wantCode:  RiskSignalRegistration1h,
		},
		{
			name:      "registration twenty four hours",
			evidence:  IPRiskEvidence{RegistrationCount24h: 10},
			wantScore: 45,
			wantCode:  RiskSignalRegistration24h,
		},
		{
			name:      "shared ua three",
			evidence:  IPRiskEvidence{MaxSharedUACount: 3},
			wantScore: 15,
			wantCode:  RiskSignalSharedUA3,
		},
		{
			name:      "shared ua five",
			evidence:  IPRiskEvidence{MaxSharedUACount: 5},
			wantScore: 20,
			wantCode:  RiskSignalSharedUA5,
		},
		{
			name:      "email pattern",
			evidence:  IPRiskEvidence{EmailPatternAccountCount: 3},
			wantScore: 15,
			wantCode:  RiskSignalEmailPattern,
		},
		{
			name:      "shared api ip",
			evidence:  IPRiskEvidence{SharedAPIIPUserCount: 3},
			wantScore: 25,
			wantCode:  RiskSignalSharedAPIIP,
		},
		{
			name:      "rapid key or gift usage across multiple accounts",
			evidence:  IPRiskEvidence{RapidKeyOrGiftUserCount: 2},
			wantScore: 15,
			wantCode:  RiskSignalRapidKeyOrGift,
		},
		{
			name:      "shared signup code",
			evidence:  IPRiskEvidence{SharedSignupCodeCount: 3},
			wantScore: 10,
			wantCode:  RiskSignalSharedSignupCode,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), testCase.evidence)
			require.Equal(t, testCase.wantScore, assessment.Score)
			require.Equal(t, []RiskSignalCode{testCase.wantCode}, signalCodes(assessment.Signals))
		})
	}
}

func TestCalculateIPRiskAssessmentRequiresMultipleAccountsForRapidBehavior(t *testing.T) {
	t.Parallel()

	assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), IPRiskEvidence{
		RapidKeyOrGiftUserCount: 1,
	})

	require.Zero(t, assessment.Score)
	require.Empty(t, assessment.Signals)
}

func TestCalculateIPRiskAssessmentCombinesIndependentSignalFamilies(t *testing.T) {
	t.Parallel()

	assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), IPRiskEvidence{
		RegistrationCount24h:     10,
		ExactRegistrationCount:   10,
		MaxSharedUACount:         5,
		EmailPatternAccountCount: 3,
		SharedAPIIPUserCount:     3,
		RapidKeyOrGiftUserCount:  3,
		SharedSignupCodeCount:    3,
		TrustedAccountCount:      1,
	})

	require.Equal(t, 100, assessment.Score)
	require.Equal(t, RiskLevelCritical, assessment.Level)
	require.Equal(t, 4, assessment.PositiveSignalFamilies)
	require.Contains(t, signalCodes(assessment.Signals), RiskSignalTrustedAccount)
}

func TestCalculateIPRiskAssessmentClampsScoreAtZero(t *testing.T) {
	t.Parallel()

	assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), IPRiskEvidence{
		TrustedAccountCount: 2,
	})

	require.Zero(t, assessment.Score)
	require.Equal(t, RiskLevelLow, assessment.Level)
}

func TestIPRiskAssessmentAutoBlockEligibility(t *testing.T) {
	t.Parallel()

	base := IPRiskEvidence{
		PrimaryIPRegistrationCount: 10,
		RegistrationCount24h:       10,
		ExactRegistrationCount:     10,
		MaxSharedUACount:           5,
		SharedAPIIPUserCount:       3,
		AllKeyEvidenceExact:        true,
		PrimaryIP:                  "203.0.113.8",
		PrimaryNetwork:             "203.0.113.8/32",
	}

	t.Run("eligible exact ipv4", func(t *testing.T) {
		assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), base)
		require.True(t, assessment.AutoBlockEligible)
		require.Equal(t, "203.0.113.8/32", assessment.AutoBlockTarget)
		require.Equal(t, 30*time.Minute, assessment.AutoBlockDuration)
	})

	t.Run("single signal family is insufficient", func(t *testing.T) {
		evidence := base
		evidence.MaxSharedUACount = 0
		evidence.SharedAPIIPUserCount = 0
		config := DefaultIPRiskConfig()
		config.AutoBlockScore = 40
		assessment := CalculateIPRiskAssessment(config, evidence)
		require.GreaterOrEqual(t, assessment.Score, config.AutoBlockScore)
		require.False(t, assessment.AutoBlockEligible)
	})

	t.Run("inferred registration evidence cannot auto block", func(t *testing.T) {
		evidence := base
		evidence.ExactRegistrationCount = 4
		evidence.AllKeyEvidenceExact = false
		assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), evidence)
		require.False(t, assessment.AutoBlockEligible)
	})

	t.Run("known shared network cannot auto block", func(t *testing.T) {
		evidence := base
		evidence.KnownSharedNetwork = true
		assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), evidence)
		require.False(t, assessment.AutoBlockEligible)
	})

	t.Run("allowlist cannot auto block", func(t *testing.T) {
		evidence := base
		evidence.Allowlisted = true
		assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), evidence)
		require.False(t, assessment.AutoBlockEligible)
	})

	t.Run("ipv6 auto block targets exact slash 128", func(t *testing.T) {
		evidence := base
		evidence.PrimaryIP = "2001:db8:7a4::19"
		evidence.PrimaryNetwork = "2001:db8:7a4::/64"
		assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), evidence)
		require.True(t, assessment.AutoBlockEligible)
		require.Equal(t, "2001:db8:7a4::19/128", assessment.AutoBlockTarget)
	})

	t.Run("ipv6 aggregate cannot borrow registrations from other slash 128 addresses", func(t *testing.T) {
		evidence := base
		evidence.PrimaryIP = "2001:db8:7a4::19"
		evidence.PrimaryNetwork = "2001:db8:7a4::/64"
		evidence.PrimaryIPRegistrationCount = 4
		assessment := CalculateIPRiskAssessment(DefaultIPRiskConfig(), evidence)
		require.False(t, assessment.AutoBlockEligible)
		require.Empty(t, assessment.AutoBlockTarget)
	})
}

func TestIPRiskPrivacyDerivations(t *testing.T) {
	t.Parallel()

	hasher := NewIPRiskHasher([]byte("unit-test-risk-hmac-key"))
	first := hasher.UserAgent("Mozilla/5.0 Chrome/148.0.0.0")
	second := hasher.UserAgent("Mozilla/5.0 Chrome/148.1.2.3")
	require.NotEmpty(t, first.Summary)
	require.Equal(t, first.Digest, second.Digest)

	patternA := hasher.EmailPattern("trial01@example.test")
	patternB := hasher.EmailPattern("trial02@example.test")
	patternDifferent := hasher.EmailPattern("owner@example.test")
	require.True(t, patternA.TemplateLike)
	require.Equal(t, patternA.Digest, patternB.Digest)
	require.NotEqual(t, patternA.Digest, patternDifferent.Digest)

	require.Equal(t, hasher.OpaqueCode("invite-123"), hasher.OpaqueCode("invite-123"))
	require.NotEqual(t, hasher.OpaqueCode("invite-123"), hasher.OpaqueCode("invite-124"))
	require.NotEqual(t, hasher.OpaqueCode("Invite-123"), hasher.OpaqueCode("invite-123"))
}

func TestIPRiskUserAgentSummaryTruncatesAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	hasher := NewIPRiskHasher([]byte("unit-test-risk-hmac-key"))
	fingerprint := hasher.UserAgent(strings.Repeat("界", 60))

	require.True(t, utf8.ValidString(fingerprint.Summary))
	require.LessOrEqual(t, len(fingerprint.Summary), 160)
	require.NotEmpty(t, fingerprint.Digest)
}

func TestIPRiskRequestMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Round(time.Second)
	want := IPRiskRequestMetadata{
		ClientIP:   "203.0.113.8",
		UserAgent:  "test-agent/1.0",
		RequestID:  "request-123",
		OccurredAt: now,
	}
	ctx := WithIPRiskRequestMetadata(context.Background(), want)
	require.Equal(t, want, IPRiskRequestMetadataFromContext(ctx))
}

func TestIPRiskEvidenceJSONUsesStableSnakeCase(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(IPRiskEvidence{
		PrimaryIP:                  "203.0.113.8",
		PrimaryNetwork:             "203.0.113.8/32",
		PrimaryIPRegistrationCount: 5,
		RegistrationCount24h:       5,
		ExactRegistrationCount:     5,
		AllKeyEvidenceExact:        true,
	})
	require.NoError(t, err)
	payload := string(raw)
	require.Contains(t, payload, `"primary_ip":"203.0.113.8"`)
	require.Contains(t, payload, `"primary_network":"203.0.113.8/32"`)
	require.Contains(t, payload, `"primary_ip_registration_count":5`)
	require.Contains(t, payload, `"registration_count_24h":5`)
	require.Contains(t, payload, `"exact_registration_count":5`)
	require.Contains(t, payload, `"all_key_evidence_exact":true`)
	require.NotContains(t, payload, `"PrimaryIP"`)
	require.NotContains(t, payload, `"RegistrationCount24h"`)
}

func signalCodes(signals []IPRiskSignal) []RiskSignalCode {
	out := make([]RiskSignalCode, 0, len(signals))
	for _, signal := range signals {
		out = append(out, signal.Code)
	}
	return out
}
