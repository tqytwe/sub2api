package service

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type AuthRiskEventType string

const (
	AuthRiskEventRegister        AuthRiskEventType = "register"
	AuthRiskEventSuccessfulLogin AuthRiskEventType = "successful_login"
	ipRiskScanLeaderLockKey                        = "ip-risk:scan"
	ipRiskScanLeaderLockTTL                        = 2 * time.Hour
)

type IPRiskUserRelation string

const (
	IPRiskUserRelationSuspectedNew    IPRiskUserRelation = "suspected_new"
	IPRiskUserRelationTrustedExisting IPRiskUserRelation = "trusted_existing"
	IPRiskUserRelationDisabled        IPRiskUserRelation = "disabled"
)

type IPRiskScanType string

const (
	IPRiskScanIncremental        IPRiskScanType = "incremental"
	IPRiskScanReconcile          IPRiskScanType = "reconcile"
	IPRiskScanDaily              IPRiskScanType = "daily"
	IPRiskScanManual             IPRiskScanType = "manual"
	IPRiskScanHistoricalBackfill IPRiskScanType = "historical_backfill"
)

type IPRiskScanStatus string

const (
	IPRiskScanPending   IPRiskScanStatus = "pending"
	IPRiskScanRunning   IPRiskScanStatus = "running"
	IPRiskScanCompleted IPRiskScanStatus = "completed"
	IPRiskScanFailed    IPRiskScanStatus = "failed"
	IPRiskScanCanceled  IPRiskScanStatus = "canceled"
)

type AuthRiskEvent struct {
	ID                   int64
	DedupeKey            string
	EventType            AuthRiskEventType
	UserID               int64
	IPAddress            string
	IPNetwork            string
	UserAgentSummary     string
	UserAgentHMAC        []byte
	EmailPatternHMAC     []byte
	EmailPatternTemplate bool
	InvitationHMAC       []byte
	AffiliateHMAC        []byte
	SignupSource         string
	RequestID            string
	EvidenceConfidence   EvidenceConfidence
	OccurredAt           time.Time
	CreatedAt            time.Time
}

type IPRiskRegistrationInput struct {
	UserID         int64
	Email          string
	SignupSource   string
	InvitationCode string
	AffiliateCode  string
}

type IPRiskRelatedUserSnapshot struct {
	UserID              int64
	RelationType        IPRiskUserRelation
	EvidenceConfidence  EvidenceConfidence
	RecommendedSelected bool
	FirstSeenAt         time.Time
	LastSeenAt          time.Time
	Evidence            map[string]any
}

type IPRiskCandidateSnapshot struct {
	Evidence           IPRiskEvidence
	EvidenceConfidence string
	Users              []IPRiskRelatedUserSnapshot
}

type IPRiskPolicyMatch struct {
	Allowlisted        bool
	KnownSharedNetwork bool
	Observed           bool
	RegistrationBlock  bool
}

type IPRiskCaseUpsert struct {
	PrimaryIP          string
	PrimaryNetwork     string
	Score              int
	Level              RiskLevel
	EvidenceConfidence string
	Signals            []IPRiskSignal
	Evidence           IPRiskEvidence
	Users              []IPRiskRelatedUserSnapshot
	RecommendedActions []string
	AutoBlockEligible  bool
	ShadowMode         bool
	DetectedAt         time.Time
}

type IPRiskCase struct {
	ID                 int64
	CaseKey            string
	PrimaryIP          string
	PrimaryNetwork     string
	Score              int
	Level              RiskLevel
	Status             string
	EvidenceConfidence string
	AutoBlockEligible  bool
	FirstDetectedAt    time.Time
	LastDetectedAt     time.Time
	Version            int64
}

type IPRiskScanCreate struct {
	ScanType    IPRiskScanType
	Status      IPRiskScanStatus
	RequestedBy *int64
	RangeStart  time.Time
	RangeEnd    time.Time
}

type IPRiskScanUpdate struct {
	Status             IPRiskScanStatus
	Progress           int
	CandidateCount     int
	CaseCount          int
	InferredEventCount int
	ErrorMessage       string
	StartedAt          *time.Time
	CompletedAt        *time.Time
}

type IPRiskScan struct {
	ID                 int64            `json:"id"`
	ScanType           IPRiskScanType   `json:"scan_type"`
	Status             IPRiskScanStatus `json:"status"`
	RequestedBy        *int64           `json:"requested_by,omitempty"`
	RangeStart         time.Time        `json:"range_start"`
	RangeEnd           time.Time        `json:"range_end"`
	Progress           int              `json:"progress"`
	CandidateCount     int              `json:"candidate_count"`
	CaseCount          int              `json:"case_count"`
	InferredEventCount int              `json:"inferred_event_count"`
	ErrorMessage       string           `json:"error_message,omitempty"`
	StartedAt          *time.Time       `json:"started_at,omitempty"`
	CompletedAt        *time.Time       `json:"completed_at,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

type IPRiskHistoricalRegistrationCandidate struct {
	AuditID    int64
	UserID     int64
	Email      string
	ClientIP   string
	UserAgent  string
	RequestID  string
	OccurredAt time.Time
}

type IPRiskHistoricalPage struct {
	Candidates  []IPRiskHistoricalRegistrationCandidate
	NextAuditID int64
	Done        bool
}

type IPRiskRegistrationThresholds struct {
	TenMinutes      int
	OneHour         int
	TwentyFourHours int
}

type IPRiskEvaluationCandidate struct {
	Network    string
	DetectedAt time.Time
}

type IPRiskRepository interface {
	InsertAuthRiskEvent(ctx context.Context, event *AuthRiskEvent) (bool, error)
	ListIPRiskEvaluationCandidates(
		ctx context.Context,
		start,
		end time.Time,
		thresholds IPRiskRegistrationThresholds,
	) ([]IPRiskEvaluationCandidate, error)
	LoadIPRiskCandidateSnapshot(ctx context.Context, network string, at time.Time) (*IPRiskCandidateSnapshot, error)
	MatchIPRiskPolicies(ctx context.Context, exactIP, network string, at time.Time) (IPRiskPolicyMatch, error)
	UpsertIPRiskCase(ctx context.Context, input *IPRiskCaseUpsert) (*IPRiskCase, error)
	CreateIPRiskScan(ctx context.Context, input *IPRiskScanCreate) (*IPRiskScan, error)
	UpdateIPRiskScan(ctx context.Context, scanID int64, update *IPRiskScanUpdate) error
	ListHistoricalRegistrationCandidates(ctx context.Context, start, end time.Time, afterAuditID int64, limit int) (*IPRiskHistoricalPage, error)
	DeleteAuthRiskEventsBefore(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)
	DeleteIPRiskRecordsBefore(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)
	LatestIPRiskScan(ctx context.Context) (*IPRiskScan, error)
	HasCompletedIPRiskScan(ctx context.Context, scanType IPRiskScanType) (bool, error)
	GetIPRiskOverview(ctx context.Context) (*IPRiskOverview, error)
	ListIPRiskCases(ctx context.Context, filter IPRiskCaseFilter) ([]IPRiskCaseSummary, int64, error)
	GetIPRiskCaseDetail(ctx context.Context, caseID int64) (*IPRiskCaseDetail, error)
	GetIPRiskManagedConfig(ctx context.Context) (*IPRiskManagedConfig, error)
	UpdateIPRiskManagedConfig(ctx context.Context, config IPRiskManagedConfig, actorID int64) error
	ListIPRiskPolicies(ctx context.Context) ([]IPRiskPolicy, error)
	CreateIPRiskPolicy(ctx context.Context, input IPRiskPolicyInput, sourceActionID *int64) (*IPRiskPolicy, error)
	UpdateIPRiskPolicy(ctx context.Context, id int64, input IPRiskPolicyInput) (*IPRiskPolicy, error)
	DeleteIPRiskPolicy(ctx context.Context, id int64) error
	GetIPRiskScan(ctx context.Context, id int64) (*IPRiskScan, error)
	ListIPRiskActions(ctx context.Context, page, pageSize int) ([]IPRiskActionRecord, int64, error)
	GetIPRiskAction(ctx context.Context, id int64) (*IPRiskActionRecord, error)
	CreateIPRiskAction(ctx context.Context, input IPRiskActionCreate) (*IPRiskActionRecord, error)
	AddIPRiskActionItem(ctx context.Context, input IPRiskActionItemCreate) error
	ReserveIPRiskActionItem(ctx context.Context, input IPRiskActionItemCreate) (int64, error)
	FinalizeIPRiskActionItem(ctx context.Context, itemID int64, targetID *int64, status, errorMessage, rollbackStatus string) error
	CompleteIPRiskAction(ctx context.Context, id int64, status string, result map[string]any, rollbackEligible bool) error
	MarkIPRiskActionRolledBack(ctx context.Context, id int64, status string) error
	UpdateIPRiskCaseStatus(ctx context.Context, id int64, status RiskCaseStatus) error
	ClaimIPRiskNotification(ctx context.Context, caseID int64, level RiskLevel) (RiskLevel, bool, error)
	RestoreIPRiskNotification(ctx context.Context, caseID int64, claimedLevel, previousLevel RiskLevel) error
}

type IPRiskRecorder interface {
	RecordRegistration(ctx context.Context, input IPRiskRegistrationInput) error
	RecordSuccessfulLogin(ctx context.Context, userID int64) error
}

type IPRiskRuntimeConfig struct {
	Enabled                    bool
	ShadowMode                 bool
	IncrementalDelay           time.Duration
	ReconcileInterval          time.Duration
	DailyScanInterval          time.Duration
	EventRetention             time.Duration
	CaseRetention              time.Duration
	HistoricalBackfillEnabled  bool
	HistoricalBackfillMaxRange time.Duration
	ManualScanMaxRange         time.Duration
	RetentionBatchSize         int
	EvaluationQueueCapacity    int
}

func DefaultIPRiskRuntimeConfig() IPRiskRuntimeConfig {
	return IPRiskRuntimeConfig{
		Enabled:                    true,
		ShadowMode:                 true,
		IncrementalDelay:           10 * time.Second,
		ReconcileInterval:          5 * time.Minute,
		DailyScanInterval:          24 * time.Hour,
		EventRetention:             90 * 24 * time.Hour,
		CaseRetention:              365 * 24 * time.Hour,
		HistoricalBackfillEnabled:  false,
		HistoricalBackfillMaxRange: 90 * 24 * time.Hour,
		ManualScanMaxRange:         90 * 24 * time.Hour,
		RetentionBatchSize:         5000,
		EvaluationQueueCapacity:    4096,
	}
}

type IPRiskRuntime struct {
	Enabled                   bool        `json:"enabled"`
	Started                   bool        `json:"started"`
	ShadowMode                bool        `json:"shadow_mode"`
	AutoBlockEnabled          bool        `json:"auto_block_enabled"`
	HistoricalBackfillEnabled bool        `json:"historical_backfill_enabled"`
	Degraded                  bool        `json:"degraded"`
	DegradedReason            string      `json:"degraded_reason,omitempty"`
	EvaluationQueueSize       int         `json:"evaluation_queue_size"`
	EvaluationQueueCapacity   int         `json:"evaluation_queue_capacity"`
	LastEvaluationAt          *time.Time  `json:"last_evaluation_at,omitempty"`
	LastScan                  *IPRiskScan `json:"last_scan,omitempty"`
	LastError                 string      `json:"last_error,omitempty"`
}

type ipRiskEvaluationRequest struct {
	network string
	dueAt   time.Time
}

type IPRiskService struct {
	repo       IPRiskRepository
	lockCache  LeaderLockCache
	db         *sql.DB
	hasher     *IPRiskHasher
	score      IPRiskConfig
	runtimeCfg IPRiskRuntimeConfig

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	queue  chan ipRiskEvaluationRequest

	started atomic.Bool

	stateMu          sync.RWMutex
	configMu         sync.RWMutex
	lastEvaluationAt *time.Time
	lastError        string
	lastEventError   string
	autoBlockEnabled bool
	alertNotifier    IPRiskAlertNotifier
	auditRecorder    IPRiskAuditRecorder
}

func NewIPRiskService(
	repo IPRiskRepository,
	lockCache LeaderLockCache,
	db *sql.DB,
	hasher *IPRiskHasher,
	runtimeCfg IPRiskRuntimeConfig,
) *IPRiskService {
	if runtimeCfg.IncrementalDelay <= 0 {
		runtimeCfg.IncrementalDelay = 10 * time.Second
	}
	if runtimeCfg.ReconcileInterval <= 0 {
		runtimeCfg.ReconcileInterval = 5 * time.Minute
	}
	if runtimeCfg.DailyScanInterval <= 0 {
		runtimeCfg.DailyScanInterval = 24 * time.Hour
	}
	if runtimeCfg.EventRetention <= 0 {
		runtimeCfg.EventRetention = 90 * 24 * time.Hour
	}
	if runtimeCfg.CaseRetention <= 0 {
		runtimeCfg.CaseRetention = 365 * 24 * time.Hour
	}
	if runtimeCfg.ManualScanMaxRange <= 0 {
		runtimeCfg.ManualScanMaxRange = 90 * 24 * time.Hour
	}
	if runtimeCfg.HistoricalBackfillMaxRange <= 0 {
		runtimeCfg.HistoricalBackfillMaxRange = 90 * 24 * time.Hour
	}
	if runtimeCfg.RetentionBatchSize <= 0 {
		runtimeCfg.RetentionBatchSize = 5000
	}
	if runtimeCfg.EvaluationQueueCapacity <= 0 {
		runtimeCfg.EvaluationQueueCapacity = 4096
	}

	// The persisted managed configuration owns automation. The migration
	// defaults it off so a fresh deployment always starts in shadow mode.
	runtimeCfg.ShadowMode = true
	ctx, cancel := context.WithCancel(context.Background())
	return &IPRiskService{
		repo:       repo,
		lockCache:  lockCache,
		db:         db,
		hasher:     hasher,
		score:      DefaultIPRiskConfig(),
		runtimeCfg: runtimeCfg,
		ctx:        ctx,
		cancel:     cancel,
		queue:      make(chan ipRiskEvaluationRequest, runtimeCfg.EvaluationQueueCapacity),
	}
}

type IPRiskAlertNotifier interface {
	NotifyIPRiskLevel(ctx context.Context, riskCase *IPRiskCase, assessment IPRiskAssessment, previousLevel RiskLevel) error
}

type IPRiskAuditRecorder interface {
	Record(entry *AuditLog)
}

func (s *IPRiskService) SetAlertNotifier(notifier IPRiskAlertNotifier) {
	if s == nil {
		return
	}
	s.configMu.Lock()
	s.alertNotifier = notifier
	s.configMu.Unlock()
}

func (s *IPRiskService) SetAuditRecorder(recorder IPRiskAuditRecorder) {
	if s == nil {
		return
	}
	s.configMu.Lock()
	s.auditRecorder = recorder
	s.configMu.Unlock()
}

func (s *IPRiskService) ApplyManagedConfig(config IPRiskManagedConfig) {
	if s == nil {
		return
	}
	s.configMu.Lock()
	s.score = IPRiskConfig{
		Registration10mThreshold:  config.Registration10mThreshold,
		Registration10mScore:      config.Registration10mScore,
		Registration1hThreshold:   config.Registration1hThreshold,
		Registration1hScore:       config.Registration1hScore,
		Registration24hThreshold:  config.Registration24hThreshold,
		Registration24hScore:      config.Registration24hScore,
		SharedUA3Threshold:        config.SharedUA3Threshold,
		SharedUA3Score:            config.SharedUA3Score,
		SharedUA5Threshold:        config.SharedUA5Threshold,
		SharedUA5Score:            config.SharedUA5Score,
		EmailPatternThreshold:     config.EmailPatternThreshold,
		EmailPatternScore:         config.EmailPatternScore,
		SharedAPIIPThreshold:      config.SharedAPIIPThreshold,
		SharedAPIIPScore:          config.SharedAPIIPScore,
		RapidBehaviorThreshold:    config.RapidBehaviorThreshold,
		RapidBehaviorScore:        config.RapidBehaviorScore,
		SharedSignupCodeThreshold: config.SharedSignupCodeThreshold,
		SharedSignupCodeScore:     config.SharedSignupCodeScore,
		TrustedAccountScore:       config.TrustedAccountScore,
		AutoBlockScore:            config.AutoBlockScore,
		AutoBlockMinRegistrations: config.AutoBlockMinRegistrations,
		AutoBlockDuration:         time.Duration(config.AutoBlockDurationMinutes) * time.Minute,
	}
	s.autoBlockEnabled = config.AutoBlockEnabled
	s.runtimeCfg.HistoricalBackfillEnabled = config.HistoricalBackfillEnabled
	s.runtimeCfg.EventRetention = time.Duration(config.EventRetentionDays) * 24 * time.Hour
	s.runtimeCfg.CaseRetention = time.Duration(config.CaseRetentionDays) * 24 * time.Hour
	s.runtimeCfg.ShadowMode = !config.AutoBlockEnabled
	s.configMu.Unlock()
}

func (s *IPRiskService) currentConfig() (IPRiskConfig, bool, bool) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.score, s.autoBlockEnabled, s.runtimeCfg.ShadowMode
}

func (s *IPRiskService) LoadManagedConfig(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return errors.New("ip risk service unavailable")
	}
	config, err := s.repo.GetIPRiskManagedConfig(ctx)
	if errors.Is(err, ErrIPRiskConfigNotFound) || errors.Is(err, sql.ErrNoRows) {
		config = func() *IPRiskManagedConfig {
			value := DefaultIPRiskManagedConfig()
			return &value
		}()
		err = nil
	}
	if err != nil {
		return err
	}
	s.ApplyManagedConfig(*config)
	return nil
}

func (s *IPRiskService) Start() {
	if s == nil || !s.runtimeCfg.Enabled || s.repo == nil || s.hasher == nil {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	if err := s.LoadManagedConfig(s.ctx); err != nil {
		s.setEvaluationState(time.Now().UTC(), "load managed config: "+err.Error())
	}
	workerCount := 2
	if s.runtimeCfg.HistoricalBackfillEnabled {
		workerCount++
	}
	s.wg.Add(workerCount)
	go s.runEvaluationLoop()
	go s.runScheduledLoop()
	if s.runtimeCfg.HistoricalBackfillEnabled {
		go s.runInitialHistoricalBackfill()
	}
}

func (s *IPRiskService) Stop() {
	if s == nil || !s.started.Load() {
		return
	}
	s.cancel()
	s.wg.Wait()
}

func (s *IPRiskService) RecordRegistration(ctx context.Context, input IPRiskRegistrationInput) error {
	if s == nil || !s.runtimeCfg.Enabled || s.repo == nil {
		return nil
	}
	if input.UserID <= 0 {
		return errors.New("ip risk registration requires user id")
	}
	metadata := IPRiskRequestMetadataFromContext(ctx)
	address, err := NormalizeIPRiskAddress(metadata.ClientIP)
	if err != nil {
		return fmt.Errorf("normalize registration ip: %w", err)
	}
	if s.hasher == nil {
		return errors.New("ip risk hmac key unavailable")
	}

	occurredAt := metadata.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	userAgent := s.hasher.UserAgent(metadata.UserAgent)
	emailPattern := s.hasher.EmailPattern(input.Email)
	var invitationHMAC []byte
	if strings.TrimSpace(input.InvitationCode) != "" {
		invitationHMAC = decodeIPRiskDigest(s.hasher.OpaqueCode("invitation:" + input.InvitationCode))
	}
	var affiliateHMAC []byte
	if strings.TrimSpace(input.AffiliateCode) != "" {
		affiliateHMAC = decodeIPRiskDigest(s.hasher.OpaqueCode("affiliate:" + input.AffiliateCode))
	}
	event := &AuthRiskEvent{
		DedupeKey:            fmt.Sprintf("register:exact:user:%d", input.UserID),
		EventType:            AuthRiskEventRegister,
		UserID:               input.UserID,
		IPAddress:            address.Exact,
		IPNetwork:            address.Network,
		UserAgentSummary:     userAgent.Summary,
		UserAgentHMAC:        decodeIPRiskDigest(userAgent.Digest),
		EmailPatternHMAC:     ipRiskEmailPatternDigest(emailPattern),
		EmailPatternTemplate: emailPattern.TemplateLike,
		InvitationHMAC:       invitationHMAC,
		AffiliateHMAC:        affiliateHMAC,
		SignupSource:         strings.TrimSpace(strings.ToLower(input.SignupSource)),
		RequestID:            strings.TrimSpace(metadata.RequestID),
		EvidenceConfidence:   EvidenceConfidenceExact,
		OccurredAt:           occurredAt,
	}
	inserted, err := s.repo.InsertAuthRiskEvent(ctx, event)
	if err != nil {
		wrapped := fmt.Errorf("insert exact registration risk event: %w", err)
		s.setEventPersistenceError(wrapped.Error())
		return wrapped
	}
	s.setEventPersistenceError("")
	if inserted {
		s.enqueueEvaluation(address.Network, occurredAt.Add(s.runtimeCfg.IncrementalDelay))
	}
	return nil
}

func (s *IPRiskService) RecordSuccessfulLogin(ctx context.Context, userID int64) error {
	if s == nil || !s.runtimeCfg.Enabled || s.repo == nil {
		return nil
	}
	if userID <= 0 {
		return errors.New("ip risk login requires user id")
	}
	metadata := IPRiskRequestMetadataFromContext(ctx)
	address, err := NormalizeIPRiskAddress(metadata.ClientIP)
	if err != nil {
		return fmt.Errorf("normalize successful login ip: %w", err)
	}
	if s.hasher == nil {
		return errors.New("ip risk hmac key unavailable")
	}
	occurredAt := metadata.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	userAgent := s.hasher.UserAgent(metadata.UserAgent)
	bucket := occurredAt.Truncate(5 * time.Minute)
	event := &AuthRiskEvent{
		DedupeKey:          fmt.Sprintf("login:%d:%s:%s:%s", userID, address.Exact, userAgent.Digest, bucket.Format(time.RFC3339)),
		EventType:          AuthRiskEventSuccessfulLogin,
		UserID:             userID,
		IPAddress:          address.Exact,
		IPNetwork:          address.Network,
		UserAgentSummary:   userAgent.Summary,
		UserAgentHMAC:      decodeIPRiskDigest(userAgent.Digest),
		RequestID:          strings.TrimSpace(metadata.RequestID),
		EvidenceConfidence: EvidenceConfidenceExact,
		OccurredAt:         occurredAt,
	}
	_, err = s.repo.InsertAuthRiskEvent(ctx, event)
	if err != nil {
		wrapped := fmt.Errorf("insert successful login risk event: %w", err)
		s.setEventPersistenceError(wrapped.Error())
		return wrapped
	}
	s.setEventPersistenceError("")
	return nil
}

func (s *IPRiskService) EvaluateNetwork(ctx context.Context, network string, detectedAt time.Time) (*IPRiskAssessment, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("ip risk repository unavailable")
	}
	network = strings.TrimSpace(network)
	if network == "" {
		return nil, errors.New("ip risk network is required")
	}
	detectedAt = detectedAt.UTC()
	if detectedAt.IsZero() {
		detectedAt = time.Now().UTC()
	}

	snapshot, err := s.repo.LoadIPRiskCandidateSnapshot(ctx, network, detectedAt)
	if err != nil {
		return nil, fmt.Errorf("load ip risk evidence: %w", err)
	}
	if snapshot == nil {
		return nil, nil
	}
	policies, err := s.repo.MatchIPRiskPolicies(
		ctx,
		snapshot.Evidence.PrimaryIP,
		snapshot.Evidence.PrimaryNetwork,
		detectedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("match ip risk policies: %w", err)
	}
	snapshot.Evidence.Allowlisted = policies.Allowlisted
	snapshot.Evidence.KnownSharedNetwork = policies.KnownSharedNetwork

	scoreConfig, autoBlockEnabled, shadowMode := s.currentConfig()
	assessment := CalculateIPRiskAssessment(scoreConfig, snapshot.Evidence)
	s.setEvaluationState(detectedAt, "")
	if assessment.Score < 40 {
		return &assessment, nil
	}

	confidence := strings.TrimSpace(snapshot.EvidenceConfidence)
	if confidence == "" {
		switch {
		case snapshot.Evidence.AllKeyEvidenceExact:
			confidence = string(EvidenceConfidenceExact)
		case snapshot.Evidence.ExactRegistrationCount == 0:
			confidence = string(EvidenceConfidenceInferred)
		default:
			confidence = "mixed"
		}
	}
	riskCase, err := s.repo.UpsertIPRiskCase(ctx, &IPRiskCaseUpsert{
		PrimaryIP:          snapshot.Evidence.PrimaryIP,
		PrimaryNetwork:     snapshot.Evidence.PrimaryNetwork,
		Score:              assessment.Score,
		Level:              assessment.Level,
		EvidenceConfidence: confidence,
		Signals:            assessment.Signals,
		Evidence:           snapshot.Evidence,
		Users:              snapshot.Users,
		RecommendedActions: recommendedIPRiskActions(assessment),
		AutoBlockEligible:  assessment.AutoBlockEligible,
		ShadowMode:         shadowMode,
		DetectedAt:         detectedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert ip risk case: %w", err)
	}
	if autoBlockEnabled && assessment.AutoBlockEligible && !policies.RegistrationBlock {
		expiresAt := detectedAt.Add(assessment.AutoBlockDuration)
		reason := fmt.Sprintf("automatic critical-risk registration block for case %d", riskCase.ID)
		action, actionErr := s.repo.CreateIPRiskAction(ctx, IPRiskActionCreate{
			CaseID:      &riskCase.ID,
			CaseVersion: riskCase.Version,
			ActionType:  RiskActionTemporaryRegistrationBan,
			ActorType:   "system",
			Reason:      reason,
			ActionSnapshot: map[string]any{
				"score":            assessment.Score,
				"target":           assessment.AutoBlockTarget,
				"duration_minutes": int(assessment.AutoBlockDuration / time.Minute),
			},
		})
		if actionErr != nil {
			return nil, fmt.Errorf("create automatic ip risk action: %w", actionErr)
		}
		itemID, itemErr := s.repo.ReserveIPRiskActionItem(ctx, IPRiskActionItemCreate{
			ActionID:    action.ID,
			TargetType:  "ip_policy",
			TargetIP:    snapshot.Evidence.PrimaryIP,
			BeforeState: map[string]any{"enabled": false},
			AfterState: map[string]any{
				"enabled":    true,
				"mode":       string(IPPolicyBlockRegistration),
				"ip_network": "",
				"exact_ip":   snapshot.Evidence.PrimaryIP,
				"reason":     reason,
				"expires_at": expiresAt.UTC().Format(time.RFC3339Nano),
			},
		})
		var policy *IPRiskPolicy
		var policyErr error
		if itemErr == nil {
			policy, policyErr = s.repo.CreateIPRiskPolicy(ctx, IPRiskPolicyInput{
				Mode:      IPPolicyBlockRegistration,
				ExactIP:   snapshot.Evidence.PrimaryIP,
				Reason:    reason,
				Enabled:   true,
				ExpiresAt: &expiresAt,
			}, &action.ID)
		} else {
			policyErr = itemErr
		}
		var targetID *int64
		if policy != nil {
			targetID = &policy.ID
		}
		itemStatus := "completed"
		if policyErr != nil {
			itemStatus = "failed"
		}
		if itemErr == nil {
			itemErr = s.repo.FinalizeIPRiskActionItem(
				ctx,
				itemID,
				targetID,
				itemStatus,
				errorString(policyErr),
				ipRiskActionItemRollbackStatus(itemStatus),
			)
		}
		if itemErr != nil {
			policyErr = errors.Join(policyErr, itemErr)
			if policy != nil {
				policyErr = errors.Join(policyErr, s.repo.DeleteIPRiskPolicy(ctx, policy.ID))
			}
		}
		actionStatus := "completed"
		completedItems := 1
		failedItems := 0
		if policyErr != nil {
			actionStatus = "failed"
			completedItems = 0
			failedItems = 1
		}
		_ = s.repo.CompleteIPRiskAction(ctx, action.ID, actionStatus, map[string]any{
			"policy_id": targetID, "completed_items": completedItems, "failed_items": failedItems,
		}, policyErr == nil)
		s.recordAutomaticBlockAudit(riskCase, action, assessment, actionStatus, policyErr)
		if policyErr != nil {
			return nil, fmt.Errorf("create automatic registration block: %w", policyErr)
		}
	}
	s.configMu.RLock()
	notifier := s.alertNotifier
	s.configMu.RUnlock()
	if notifier != nil && riskCase != nil && assessment.Score >= 80 {
		previousLevel, claimed, claimErr := s.repo.ClaimIPRiskNotification(ctx, riskCase.ID, assessment.Level)
		if claimErr != nil {
			logger.LegacyPrintf("service.ip_risk", "[IPRisk] alert claim failed: case_id=%d err=%v", riskCase.ID, claimErr)
		} else if claimed {
			if notifyErr := notifier.NotifyIPRiskLevel(ctx, riskCase, assessment, previousLevel); notifyErr != nil {
				if restoreErr := s.repo.RestoreIPRiskNotification(ctx, riskCase.ID, assessment.Level, previousLevel); restoreErr != nil {
					notifyErr = errors.Join(notifyErr, restoreErr)
				}
				logger.LegacyPrintf("service.ip_risk", "[IPRisk] alert notification failed: case_id=%d err=%v", riskCase.ID, notifyErr)
			}
		}
	}
	return &assessment, nil
}

func (s *IPRiskService) recordAutomaticBlockAudit(
	riskCase *IPRiskCase,
	action *IPRiskActionRecord,
	assessment IPRiskAssessment,
	status string,
	actionErr error,
) {
	if s == nil {
		return
	}
	s.configMu.RLock()
	recorder := s.auditRecorder
	s.configMu.RUnlock()
	if recorder == nil || riskCase == nil || action == nil {
		return
	}
	statusCode := 200
	if actionErr != nil {
		statusCode = 500
	}
	recorder.Record(&AuditLog{
		CreatedAt:  time.Now().UTC(),
		ActorRole:  "system",
		AuthMethod: "system",
		Action:     AuditActionSystemIPRiskRegistrationBlock,
		Method:     "SYSTEM",
		Path:       "/internal/ip-risk/automatic-registration-block",
		StatusCode: statusCode,
		Extra: map[string]any{
			"result":           status,
			"case_id":          riskCase.ID,
			"action_id":        action.ID,
			"risk_score":       assessment.Score,
			"risk_level":       string(assessment.Level),
			"duration_minutes": int(assessment.AutoBlockDuration / time.Minute),
		},
	})
}

func recommendedIPRiskActions(assessment IPRiskAssessment) []string {
	switch assessment.Level {
	case RiskLevelCritical:
		return []string{"temporary_registration_block", "review_related_users"}
	case RiskLevelSevere, RiskLevelHigh:
		return []string{"review_related_users", "consider_key_pause"}
	case RiskLevelMedium:
		return []string{"observe"}
	default:
		return nil
	}
}

func (s *IPRiskService) RunScan(
	ctx context.Context,
	scanType IPRiskScanType,
	start,
	end time.Time,
	requestedBy *int64,
) (*IPRiskScan, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("ip risk repository unavailable")
	}
	start = start.UTC()
	end = end.UTC()
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil, errors.New("invalid ip risk scan range")
	}
	if scanType == IPRiskScanManual && end.Sub(start) > s.runtimeCfg.ManualScanMaxRange {
		return nil, fmt.Errorf("manual ip risk scan range exceeds %s", s.runtimeCfg.ManualScanMaxRange)
	}

	owner := fmt.Sprintf("%d", time.Now().UnixNano())
	release, acquired := tryAcquireSingletonLeaderLock(
		ctx,
		s.lockCache,
		s.db,
		ipRiskScanLeaderLockKey,
		owner,
		ipRiskScanLeaderLockTTL,
	)
	if !acquired {
		return nil, errors.New("ip risk scan is already running")
	}
	defer release()

	now := time.Now().UTC()
	scan, err := s.repo.CreateIPRiskScan(ctx, &IPRiskScanCreate{
		ScanType:    scanType,
		Status:      IPRiskScanRunning,
		RequestedBy: requestedBy,
		RangeStart:  start,
		RangeEnd:    end,
	})
	if err != nil {
		return nil, fmt.Errorf("create ip risk scan: %w", err)
	}
	startedAt := now
	_ = s.repo.UpdateIPRiskScan(ctx, scan.ID, &IPRiskScanUpdate{
		Status:    IPRiskScanRunning,
		StartedAt: &startedAt,
	})

	fail := func(
		runErr error,
		candidateCount,
		processedCount,
		caseCount,
		inferredCount int,
	) (*IPRiskScan, error) {
		completedAt := time.Now().UTC()
		status := IPRiskScanFailed
		if errors.Is(runErr, context.Canceled) {
			status = IPRiskScanCanceled
		}
		progress := 0
		if candidateCount > 0 {
			progress = processedCount * 100 / candidateCount
		}
		updateCtx := ctx
		cancelUpdate := func() {}
		if ctx.Err() != nil {
			updateCtx, cancelUpdate = context.WithTimeout(context.Background(), 5*time.Second)
		}
		defer cancelUpdate()
		_ = s.repo.UpdateIPRiskScan(updateCtx, scan.ID, &IPRiskScanUpdate{
			Status:             status,
			Progress:           progress,
			CandidateCount:     candidateCount,
			CaseCount:          caseCount,
			InferredEventCount: inferredCount,
			ErrorMessage:       runErr.Error(),
			CompletedAt:        &completedAt,
		})
		scan.Status = status
		scan.Progress = progress
		scan.CandidateCount = candidateCount
		scan.CaseCount = caseCount
		scan.InferredEventCount = inferredCount
		scan.ErrorMessage = runErr.Error()
		scan.StartedAt = &startedAt
		scan.CompletedAt = &completedAt
		s.setEvaluationState(completedAt, runErr.Error())
		return scan, runErr
	}

	inferredCount := 0
	if scanType == IPRiskScanHistoricalBackfill {
		inferredCount, err = s.InferHistoricalRegistrations(ctx, start, end)
		if err != nil {
			return fail(err, 0, 0, 0, inferredCount)
		}
	}

	scoreConfig, _, _ := s.currentConfig()
	candidates, err := s.repo.ListIPRiskEvaluationCandidates(ctx, start, end, IPRiskRegistrationThresholds{
		TenMinutes:      scoreConfig.Registration10mThreshold,
		OneHour:         scoreConfig.Registration1hThreshold,
		TwentyFourHours: scoreConfig.Registration24hThreshold,
	})
	if err != nil {
		return fail(fmt.Errorf("list ip risk candidates: %w", err), 0, 0, 0, inferredCount)
	}
	caseCount := 0
	for index, candidate := range candidates {
		assessment, evaluateErr := s.EvaluateNetwork(ctx, candidate.Network, candidate.DetectedAt)
		if evaluateErr != nil {
			return fail(evaluateErr, len(candidates), index, caseCount, inferredCount)
		}
		if assessment != nil && assessment.Score >= 40 {
			caseCount++
		}
		progress := 100
		if len(candidates) > 0 {
			progress = (index + 1) * 100 / len(candidates)
		}
		_ = s.repo.UpdateIPRiskScan(ctx, scan.ID, &IPRiskScanUpdate{
			Status:             IPRiskScanRunning,
			Progress:           progress,
			CandidateCount:     len(candidates),
			CaseCount:          caseCount,
			InferredEventCount: inferredCount,
		})
	}

	if err := ctx.Err(); err != nil {
		return fail(err, len(candidates), len(candidates), caseCount, inferredCount)
	}
	completedAt := time.Now().UTC()
	if err := s.repo.UpdateIPRiskScan(ctx, scan.ID, &IPRiskScanUpdate{
		Status:             IPRiskScanCompleted,
		Progress:           100,
		CandidateCount:     len(candidates),
		CaseCount:          caseCount,
		InferredEventCount: inferredCount,
		CompletedAt:        &completedAt,
	}); err != nil {
		return scan, fmt.Errorf("complete ip risk scan: %w", err)
	}
	scan.Status = IPRiskScanCompleted
	scan.Progress = 100
	scan.CandidateCount = len(candidates)
	scan.CaseCount = caseCount
	scan.InferredEventCount = inferredCount
	scan.StartedAt = &startedAt
	scan.CompletedAt = &completedAt
	s.setEvaluationState(completedAt, "")
	return scan, nil
}

func (s *IPRiskService) StartScan(
	ctx context.Context,
	scanType IPRiskScanType,
	start,
	end time.Time,
	requestedBy *int64,
) (*IPRiskScan, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("ip risk scanner unavailable")
	}
	start = start.UTC()
	end = end.UTC()
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil, errors.New("invalid ip risk scan range")
	}
	if scanType == IPRiskScanManual && end.Sub(start) > s.runtimeCfg.ManualScanMaxRange {
		return nil, fmt.Errorf("manual ip risk scan range exceeds %s", s.runtimeCfg.ManualScanMaxRange)
	}
	scan, err := s.repo.CreateIPRiskScan(ctx, &IPRiskScanCreate{
		ScanType:    scanType,
		Status:      IPRiskScanPending,
		RequestedBy: requestedBy,
		RangeStart:  start,
		RangeEnd:    end,
	})
	if err != nil {
		return nil, err
	}
	go func(scanID int64) {
		runCtx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
		defer cancel()
		if err := s.runExistingScan(runCtx, scanID, scanType, start, end, requestedBy); err != nil {
			logger.LegacyPrintf("service.ip_risk", "[IPRisk] async scan failed: scan_id=%d err=%v", scanID, err)
		}
	}(scan.ID)
	return scan, nil
}

func (s *IPRiskService) runExistingScan(
	ctx context.Context,
	scanID int64,
	scanType IPRiskScanType,
	start,
	end time.Time,
	requestedBy *int64,
) error {
	owner := fmt.Sprintf("%d", time.Now().UnixNano())
	release, acquired := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, ipRiskScanLeaderLockKey, owner, ipRiskScanLeaderLockTTL)
	if !acquired {
		completedAt := time.Now().UTC()
		return s.repo.UpdateIPRiskScan(ctx, scanID, &IPRiskScanUpdate{
			Status:       IPRiskScanFailed,
			ErrorMessage: "ip risk scan is already running",
			CompletedAt:  &completedAt,
		})
	}
	defer release()
	startedAt := time.Now().UTC()
	if err := s.repo.UpdateIPRiskScan(ctx, scanID, &IPRiskScanUpdate{
		Status:    IPRiskScanRunning,
		StartedAt: &startedAt,
	}); err != nil {
		return err
	}
	inferredCount := 0
	if scanType == IPRiskScanHistoricalBackfill {
		value, err := s.InferHistoricalRegistrations(ctx, start, end)
		inferredCount = value
		if err != nil {
			return s.failExistingScan(ctx, scanID, err, 0, 0, 0, inferredCount)
		}
	}
	scoreConfig, _, _ := s.currentConfig()
	candidates, err := s.repo.ListIPRiskEvaluationCandidates(ctx, start, end, IPRiskRegistrationThresholds{
		TenMinutes:      scoreConfig.Registration10mThreshold,
		OneHour:         scoreConfig.Registration1hThreshold,
		TwentyFourHours: scoreConfig.Registration24hThreshold,
	})
	if err != nil {
		return s.failExistingScan(ctx, scanID, err, 0, 0, 0, inferredCount)
	}
	caseCount := 0
	for index, candidate := range candidates {
		assessment, evaluateErr := s.EvaluateNetwork(ctx, candidate.Network, candidate.DetectedAt)
		if evaluateErr != nil {
			return s.failExistingScan(ctx, scanID, evaluateErr, len(candidates), index, caseCount, inferredCount)
		}
		if assessment != nil && assessment.Score >= 40 {
			caseCount++
		}
		progress := 100
		if len(candidates) > 0 {
			progress = (index + 1) * 100 / len(candidates)
		}
		_ = s.repo.UpdateIPRiskScan(ctx, scanID, &IPRiskScanUpdate{
			Status:             IPRiskScanRunning,
			Progress:           progress,
			CandidateCount:     len(candidates),
			CaseCount:          caseCount,
			InferredEventCount: inferredCount,
			StartedAt:          &startedAt,
		})
	}
	completedAt := time.Now().UTC()
	return s.repo.UpdateIPRiskScan(ctx, scanID, &IPRiskScanUpdate{
		Status:             IPRiskScanCompleted,
		Progress:           100,
		CandidateCount:     len(candidates),
		CaseCount:          caseCount,
		InferredEventCount: inferredCount,
		StartedAt:          &startedAt,
		CompletedAt:        &completedAt,
	})
}

func (s *IPRiskService) failExistingScan(ctx context.Context, scanID int64, runErr error, candidateCount, processedCount, caseCount, inferredCount int) error {
	completedAt := time.Now().UTC()
	progress := 0
	if candidateCount > 0 {
		progress = processedCount * 100 / candidateCount
	}
	status := IPRiskScanFailed
	if errors.Is(runErr, context.Canceled) {
		status = IPRiskScanCanceled
	}
	updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	updateErr := s.repo.UpdateIPRiskScan(updateCtx, scanID, &IPRiskScanUpdate{
		Status:             status,
		Progress:           progress,
		CandidateCount:     candidateCount,
		CaseCount:          caseCount,
		InferredEventCount: inferredCount,
		ErrorMessage:       runErr.Error(),
		CompletedAt:        &completedAt,
	})
	if updateErr != nil {
		return errors.Join(runErr, updateErr)
	}
	return runErr
}

var ErrIPRiskRegistrationBlocked = errors.New("registration temporarily blocked for this IP")

func (s *IPRiskService) CheckRegistrationAllowed(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return nil
	}
	metadata := IPRiskRequestMetadataFromContext(ctx)
	address, err := NormalizeIPRiskAddress(metadata.ClientIP)
	if err != nil {
		return nil
	}
	match, err := s.repo.MatchIPRiskPolicies(ctx, address.Exact, address.Network, time.Now().UTC())
	if err != nil {
		s.setEventPersistenceError("registration policy check: " + err.Error())
		return nil
	}
	if match.Allowlisted || match.KnownSharedNetwork {
		return nil
	}
	if match.RegistrationBlock {
		return ErrIPRiskRegistrationBlocked
	}
	return nil
}

func (s *IPRiskService) InferHistoricalRegistrations(ctx context.Context, start, end time.Time) (int, error) {
	if s == nil || s.repo == nil || s.hasher == nil {
		return 0, errors.New("ip risk historical inference unavailable")
	}
	if end.Before(start) || end.Sub(start) > s.runtimeCfg.HistoricalBackfillMaxRange {
		return 0, errors.New("invalid ip risk historical inference range")
	}

	insertedCount := 0
	var afterAuditID int64
	for {
		page, err := s.repo.ListHistoricalRegistrationCandidates(ctx, start, end, afterAuditID, 500)
		if err != nil {
			return insertedCount, fmt.Errorf("list historical registration candidates: %w", err)
		}
		if page == nil {
			return insertedCount, nil
		}
		for _, candidate := range page.Candidates {
			address, err := NormalizeIPRiskAddress(candidate.ClientIP)
			if err != nil || candidate.UserID <= 0 {
				continue
			}
			ua := s.hasher.UserAgent(candidate.UserAgent)
			emailPattern := s.hasher.EmailPattern(candidate.Email)
			inserted, err := s.repo.InsertAuthRiskEvent(ctx, &AuthRiskEvent{
				DedupeKey:            fmt.Sprintf("register:inferred:audit:%d", candidate.AuditID),
				EventType:            AuthRiskEventRegister,
				UserID:               candidate.UserID,
				IPAddress:            address.Exact,
				IPNetwork:            address.Network,
				UserAgentSummary:     ua.Summary,
				UserAgentHMAC:        decodeIPRiskDigest(ua.Digest),
				EmailPatternHMAC:     ipRiskEmailPatternDigest(emailPattern),
				EmailPatternTemplate: emailPattern.TemplateLike,
				SignupSource:         "email",
				RequestID:            strings.TrimSpace(candidate.RequestID),
				EvidenceConfidence:   EvidenceConfidenceInferred,
				OccurredAt:           candidate.OccurredAt.UTC(),
			})
			if err != nil {
				return insertedCount, fmt.Errorf("insert inferred registration event: %w", err)
			}
			if inserted {
				insertedCount++
			}
		}
		if page.Done || page.NextAuditID <= afterAuditID {
			break
		}
		afterAuditID = page.NextAuditID
	}
	return insertedCount, nil
}

func (s *IPRiskService) Runtime(ctx context.Context) IPRiskRuntime {
	_, autoBlockEnabled, shadowMode := s.currentConfig()
	runtime := IPRiskRuntime{
		Enabled:          s != nil && s.runtimeCfg.Enabled,
		ShadowMode:       shadowMode,
		AutoBlockEnabled: autoBlockEnabled,
	}
	if s == nil {
		runtime.Degraded = true
		runtime.DegradedReason = "service unavailable"
		return runtime
	}
	if s.queue != nil {
		runtime.EvaluationQueueSize = len(s.queue)
		runtime.EvaluationQueueCapacity = cap(s.queue)
	}
	runtime.Started = s.started.Load()
	runtime.HistoricalBackfillEnabled = s.runtimeCfg.HistoricalBackfillEnabled
	if runtime.Enabled && !runtime.Started {
		runtime.Degraded = true
		runtime.DegradedReason = "scanner not started"
	}
	if s.repo == nil {
		runtime.Degraded = true
		runtime.DegradedReason = "repository unavailable"
	}
	if s.hasher == nil {
		runtime.Degraded = true
		runtime.DegradedReason = "hmac key unavailable"
	}
	s.stateMu.RLock()
	if s.lastEvaluationAt != nil {
		value := *s.lastEvaluationAt
		runtime.LastEvaluationAt = &value
	}
	runtime.LastError = s.lastError
	if s.lastEventError != "" {
		if runtime.LastError == "" {
			runtime.LastError = s.lastEventError
		} else if runtime.LastError != s.lastEventError {
			runtime.LastError += "; " + s.lastEventError
		}
	}
	s.stateMu.RUnlock()
	if runtime.LastError != "" {
		runtime.Degraded = true
		if runtime.DegradedReason == "" {
			runtime.DegradedReason = "runtime error"
		}
	}
	if s.repo != nil {
		if scan, err := s.repo.LatestIPRiskScan(ctx); err == nil {
			runtime.LastScan = scan
			if scan.Status == IPRiskScanFailed {
				runtime.Degraded = true
				if runtime.DegradedReason == "" {
					runtime.DegradedReason = "latest scan failed"
				}
				if runtime.LastError == "" {
					runtime.LastError = scan.ErrorMessage
				}
			}
		} else if !errors.Is(err, sql.ErrNoRows) && runtime.LastError == "" {
			runtime.LastError = err.Error()
			runtime.Degraded = true
			if runtime.DegradedReason == "" {
				runtime.DegradedReason = "scan state unavailable"
			}
		}
	}
	return runtime
}

func (s *IPRiskService) enqueueEvaluation(network string, dueAt time.Time) {
	if s == nil || !s.started.Load() || strings.TrimSpace(network) == "" {
		return
	}
	select {
	case s.queue <- ipRiskEvaluationRequest{network: network, dueAt: dueAt.UTC()}:
	default:
		s.setEvaluationState(time.Now().UTC(), "incremental evaluation queue full")
	}
}

func (s *IPRiskService) runEvaluationLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	pending := make(map[string]time.Time)
	for {
		select {
		case <-s.ctx.Done():
			return
		case request := <-s.queue:
			if existing, ok := pending[request.network]; !ok || request.dueAt.Before(existing) {
				pending[request.network] = request.dueAt
			}
		case now := <-ticker.C:
			for network, dueAt := range pending {
				if now.Before(dueAt) {
					continue
				}
				ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
				_, err := s.EvaluateNetwork(ctx, network, now.UTC())
				cancel()
				if err != nil {
					s.setEvaluationState(now.UTC(), err.Error())
					logger.LegacyPrintf("service.ip_risk", "[IPRisk] incremental evaluation failed: network=%s err=%v", network, err)
				}
				delete(pending, network)
			}
		}
	}
}

func (s *IPRiskService) runScheduledLoop() {
	defer s.wg.Done()
	reconcileTicker := time.NewTicker(s.runtimeCfg.ReconcileInterval)
	defer reconcileTicker.Stop()
	dailyTicker := time.NewTicker(s.runtimeCfg.DailyScanInterval)
	defer dailyTicker.Stop()
	retentionTicker := time.NewTicker(24 * time.Hour)
	defer retentionTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-reconcileTicker.C:
			s.runScheduledScan(IPRiskScanReconcile, now.Add(-24*time.Hour), now)
		case now := <-dailyTicker.C:
			s.runScheduledScan(IPRiskScanDaily, now.Add(-30*24*time.Hour), now)
		case now := <-retentionTicker.C:
			s.runRetention(now.UTC())
		}
	}
}

func (s *IPRiskService) runInitialHistoricalBackfill() {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
	defer cancel()

	completed, err := s.repo.HasCompletedIPRiskScan(ctx, IPRiskScanHistoricalBackfill)
	if err != nil {
		s.setEvaluationState(time.Now().UTC(), err.Error())
		logger.LegacyPrintf("service.ip_risk", "[IPRisk] historical backfill state check failed: err=%v", err)
		return
	}
	if completed {
		return
	}

	now := time.Now().UTC()
	if _, err := s.RunScan(
		ctx,
		IPRiskScanHistoricalBackfill,
		now.Add(-s.runtimeCfg.HistoricalBackfillMaxRange),
		now,
		nil,
	); err != nil && !strings.Contains(err.Error(), "already running") {
		s.setEvaluationState(time.Now().UTC(), err.Error())
		logger.LegacyPrintf("service.ip_risk", "[IPRisk] historical backfill failed: err=%v", err)
	}
}

func (s *IPRiskService) runScheduledScan(scanType IPRiskScanType, start, end time.Time) {
	ctx, cancel := context.WithTimeout(s.ctx, 20*time.Minute)
	defer cancel()
	if _, err := s.RunScan(ctx, scanType, start, end, nil); err != nil &&
		!strings.Contains(err.Error(), "already running") {
		s.setEvaluationState(time.Now().UTC(), err.Error())
		logger.LegacyPrintf("service.ip_risk", "[IPRisk] scheduled scan failed: type=%s err=%v", scanType, err)
	}
}

func (s *IPRiskService) runRetention(now time.Time) {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
	defer cancel()
	for {
		deleted, err := s.repo.DeleteAuthRiskEventsBefore(ctx, now.Add(-s.runtimeCfg.EventRetention), s.runtimeCfg.RetentionBatchSize)
		if err != nil {
			s.setEvaluationState(now, err.Error())
			return
		}
		if deleted < int64(s.runtimeCfg.RetentionBatchSize) {
			break
		}
	}
	for {
		deleted, err := s.repo.DeleteIPRiskRecordsBefore(ctx, now.Add(-s.runtimeCfg.CaseRetention), s.runtimeCfg.RetentionBatchSize)
		if err != nil {
			s.setEvaluationState(now, err.Error())
			return
		}
		if deleted < int64(s.runtimeCfg.RetentionBatchSize) {
			break
		}
	}
}

func (s *IPRiskService) setEvaluationState(at time.Time, errText string) {
	if s == nil {
		return
	}
	s.stateMu.Lock()
	value := at.UTC()
	s.lastEvaluationAt = &value
	s.lastError = strings.TrimSpace(errText)
	s.stateMu.Unlock()
}

func (s *IPRiskService) setEventPersistenceError(errText string) {
	if s == nil {
		return
	}
	s.stateMu.Lock()
	s.lastEventError = strings.TrimSpace(errText)
	s.stateMu.Unlock()
}

func decodeIPRiskDigest(value string) []byte {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil
	}
	return decoded
}

func ipRiskEmailPatternDigest(pattern IPRiskEmailPattern) []byte {
	if !pattern.TemplateLike {
		return nil
	}
	return decodeIPRiskDigest(pattern.Digest)
}
