package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMobilePushPollInterval = 2 * time.Second
	defaultMobilePushClaimLease   = 2 * time.Minute
	defaultMobilePushBatchSize    = 50
	defaultMobilePushMaxAttempts  = 8
)

// MobilePushConfig is intentionally separate from Config until the P2 push
// providers are added to the main dependency graph. Credential material must
// never be marshaled or logged.
type MobilePushConfig struct {
	Enabled            bool
	ProjectID          string
	ServiceAccountFile string
	ServiceAccountJSON string `json:"-"`
	APIBaseURL         string
	PollInterval       time.Duration
	ClaimLease         time.Duration
	BatchSize          int
	MaxAttempts        int
}

func LoadMobilePushConfigFromEnv() MobilePushConfig {
	return MobilePushConfig{
		Enabled:            mobilePushEnvBool("MOBILE_PUSH_ENABLED", false),
		ProjectID:          strings.TrimSpace(os.Getenv("FCM_PROJECT_ID")),
		ServiceAccountFile: strings.TrimSpace(os.Getenv("FCM_SERVICE_ACCOUNT_FILE")),
		ServiceAccountJSON: strings.TrimSpace(os.Getenv("FCM_SERVICE_ACCOUNT_JSON")),
		APIBaseURL:         strings.TrimSpace(os.Getenv("FCM_API_BASE_URL")),
		PollInterval:       mobilePushEnvDuration("MOBILE_PUSH_POLL_INTERVAL", defaultMobilePushPollInterval),
		ClaimLease:         mobilePushEnvDuration("MOBILE_PUSH_CLAIM_LEASE", defaultMobilePushClaimLease),
		BatchSize:          mobilePushEnvInt("MOBILE_PUSH_BATCH_SIZE", defaultMobilePushBatchSize),
		MaxAttempts:        mobilePushEnvInt("MOBILE_PUSH_MAX_ATTEMPTS", defaultMobilePushMaxAttempts),
	}
}

func (c MobilePushConfig) Normalized() MobilePushConfig {
	c.ProjectID = strings.TrimSpace(c.ProjectID)
	c.ServiceAccountFile = strings.TrimSpace(c.ServiceAccountFile)
	c.ServiceAccountJSON = strings.TrimSpace(c.ServiceAccountJSON)
	c.APIBaseURL = strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if c.PollInterval <= 0 {
		c.PollInterval = defaultMobilePushPollInterval
	}
	if c.ClaimLease <= 0 {
		c.ClaimLease = defaultMobilePushClaimLease
	}
	if c.BatchSize <= 0 || c.BatchSize > 500 {
		c.BatchSize = defaultMobilePushBatchSize
	}
	if c.MaxAttempts <= 0 || c.MaxAttempts > 100 {
		c.MaxAttempts = defaultMobilePushMaxAttempts
	}
	return c
}

func (c MobilePushConfig) HasCredentials() bool {
	c = c.Normalized()
	return c.Enabled && (c.ServiceAccountFile != "" || c.ServiceAccountJSON != "")
}

func (c MobilePushConfig) HasAnyCredentialSetting() bool {
	c = c.Normalized()
	return c.Enabled && (c.ProjectID != "" || c.ServiceAccountFile != "" || c.ServiceAccountJSON != "")
}

func (c MobilePushConfig) ValidateCredentials() error {
	c = c.Normalized()
	if !c.HasAnyCredentialSetting() {
		return nil
	}
	if c.ServiceAccountFile == "" && c.ServiceAccountJSON == "" {
		return errors.New("mobile push credentials require FCM_SERVICE_ACCOUNT_FILE or FCM_SERVICE_ACCOUNT_JSON")
	}
	if c.ServiceAccountFile != "" && c.ServiceAccountJSON != "" {
		return errors.New("mobile push credentials must use only one FCM service account source")
	}
	return nil
}

func mobilePushEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func mobilePushEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func mobilePushEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
