package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mobileVideoWorkerTaskStoreFake struct {
	task        MobileTask
	transitions []MobileTaskTransitionInput
	finalized   bool
}

func (s *mobileVideoWorkerTaskStoreFake) Get(context.Context, int64, string) (*MobileTask, error) {
	copy := s.task
	copy.Artifacts = append([]MobileTaskArtifact(nil), s.task.Artifacts...)
	return &copy, nil
}

func (s *mobileVideoWorkerTaskStoreFake) Transition(_ context.Context, _ int64, _ string, input MobileTaskTransitionInput) (*MobileTask, error) {
	s.transitions = append(s.transitions, input)
	if input.Status != s.task.Status {
		updated, err := TransitionMobileTask(s.task, input.Status, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		s.task = updated
	}
	if input.Progress != nil {
		s.task.Progress = *input.Progress
	}
	if input.Artifacts != nil {
		s.task.Artifacts = append([]MobileTaskArtifact(nil), input.Artifacts...)
	}
	s.task.Error = input.Error
	return &s.task, nil
}

func (s *mobileVideoWorkerTaskStoreFake) FinalizeVideoTask(_ context.Context, _ int64, _ string, input MobileVideoTaskFinalizeInput) (*MobileTask, error) {
	if s.task.Status == MobileTaskStatusCancelled {
		return &s.task, ErrMobileVideoTaskCancelled
	}
	if s.task.Status == MobileTaskStatusQueued {
		updated, err := TransitionMobileTask(s.task, MobileTaskStatusRunning, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		s.task = updated
	}
	s.task.Artifacts = []MobileTaskArtifact{input.Artifact}
	updated, err := TransitionMobileTask(s.task, MobileTaskStatusCompleted, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	s.task = updated
	s.finalized = true
	return &s.task, nil
}

type mobileVideoWorkerJobStoreFake struct {
	job             *MobileVideoJob
	submitted       bool
	completed       bool
	failed          bool
	cancelled       bool
	billingCaptured bool
	billingReleased bool
	discarded       bool
}

func (s *mobileVideoWorkerJobStoreFake) Create(context.Context, int64, string, MobileVideoJobCreateInput) error {
	return nil
}

func (s *mobileVideoWorkerJobStoreFake) Get(context.Context, int64, string) (*MobileVideoJob, error) {
	if s.job == nil {
		return nil, ErrMobileVideoJobNotFound
	}
	copy := *s.job
	return &copy, nil
}

func (s *mobileVideoWorkerJobStoreFake) ClaimNext(context.Context, string, time.Time, time.Duration) (*MobileVideoJob, error) {
	if s.job == nil || s.completed || s.failed || s.cancelled {
		return nil, ErrMobileVideoJobNotFound
	}
	switch s.job.State {
	case MobileVideoJobStateQueued, MobileVideoJobStatePolling, MobileVideoJobStateSubmissionUnknown:
	default:
		return nil, ErrMobileVideoJobNotFound
	}
	copy := *s.job
	copy.LeaseOwner = "worker"
	if s.submitted {
		copy.ProviderRequestID = "provider-1"
	}
	return &copy, nil
}

func (s *mobileVideoWorkerJobStoreFake) ClaimFundingRelease(context.Context, string, time.Time, time.Duration) (*MobileVideoJob, error) {
	if s.job == nil || s.job.State != MobileVideoJobStateFunding || s.job.BillingState != MobileVideoBillingStateReleasing || s.discarded {
		return nil, ErrMobileVideoJobNotFound
	}
	copy := *s.job
	copy.LeaseOwner = "worker"
	return &copy, nil
}

func (s *mobileVideoWorkerJobStoreFake) Heartbeat(context.Context, string, string, time.Time, time.Duration) error {
	return nil
}

func (s *mobileVideoWorkerJobStoreFake) MarkSubmitted(_ context.Context, _ string, _ string, providerRequestID string, _ time.Time) error {
	s.submitted = true
	s.job.ProviderRequestID = providerRequestID
	s.job.State = MobileVideoJobStatePolling
	return nil
}

func (*mobileVideoWorkerJobStoreFake) MarkSubmissionUnknown(context.Context, string, string, string) error {
	return nil
}

func (s *mobileVideoWorkerJobStoreFake) MarkCompleted(context.Context, string, string, string, string, int64) error {
	s.completed = true
	s.job.State = MobileVideoJobStateCompleted
	return nil
}

func (s *mobileVideoWorkerJobStoreFake) MarkFailed(context.Context, string, string, string, string, bool) error {
	s.failed = true
	s.job.State = MobileVideoJobStateFailed
	return nil
}

func (*mobileVideoWorkerJobStoreFake) MarkClientCancelled(context.Context, string) error { return nil }
func (*mobileVideoWorkerJobStoreFake) MarkFundingReserved(context.Context, string) error { return nil }
func (*mobileVideoWorkerJobStoreFake) MarkFundingReleasePending(context.Context, string) error {
	return nil
}
func (s *mobileVideoWorkerJobStoreFake) MarkFundingReleased(context.Context, string) error {
	s.job.BillingState = MobileVideoBillingStateReleased
	return nil
}
func (s *mobileVideoWorkerJobStoreFake) MarkBillingCaptured(context.Context, string, string) error {
	s.billingCaptured = true
	s.job.BillingState = MobileVideoBillingStateCaptured
	return nil
}
func (s *mobileVideoWorkerJobStoreFake) MarkBillingReleasePending(context.Context, string, string) error {
	s.job.BillingState = MobileVideoBillingStateReleasing
	return nil
}
func (s *mobileVideoWorkerJobStoreFake) MarkBillingReleased(context.Context, string, string) error {
	s.billingReleased = true
	s.job.BillingState = MobileVideoBillingStateReleased
	return nil
}
func (*mobileVideoWorkerJobStoreFake) ActivateReserved(context.Context, int64, string, int) error {
	return nil
}
func (s *mobileVideoWorkerJobStoreFake) DiscardUnfunded(context.Context, int64, string) error {
	s.discarded = true
	s.job = nil
	return nil
}

func (s *mobileVideoWorkerJobStoreFake) MarkCancelled(context.Context, string, string) error {
	s.cancelled = true
	s.job.State = MobileVideoJobStateCancelled
	return nil
}

func (s *mobileVideoWorkerJobStoreFake) ReleaseArtifact(context.Context, int64, string) error {
	return nil
}

type mobileVideoWorkerProviderFake struct {
	creates      int
	polls        int
	createResult MobileVideoProviderResult
	pollResult   MobileVideoProviderResult
	createErr    error
	pollErr      error
}

func (p *mobileVideoWorkerProviderFake) Create(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	p.creates++
	if p.createErr != nil {
		return MobileVideoProviderResult{}, p.createErr
	}
	if p.createResult.Status != "" || p.createResult.RequestID != "" {
		return p.createResult, nil
	}
	return MobileVideoProviderResult{RequestID: "provider-1", Status: "queued"}, nil
}

func (p *mobileVideoWorkerProviderFake) Poll(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	p.polls++
	if p.pollErr != nil {
		return MobileVideoProviderResult{}, p.pollErr
	}
	if p.pollResult.Status != "" || p.pollResult.RequestID != "" || len(p.pollResult.Artifact) > 0 {
		return p.pollResult, nil
	}
	return MobileVideoProviderResult{RequestID: "provider-1", Status: "completed", Artifact: []byte("video-bytes"), ContentType: "video/mp4"}, nil
}

func (p *mobileVideoWorkerProviderFake) Content(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	return MobileVideoProviderResult{}, errors.New("content should not be requested when poll includes bytes")
}

type mobileVideoWorkerStorageFake struct{}

func (mobileVideoWorkerStorageFake) Save(context.Context, string, string, []byte) (string, error) {
	return "https://storage.example/private/video.mp4", nil
}

type mobileVideoWorkerBillingFake struct {
	reserveCalls int
	captureCalls int
	releaseCalls int
	reserveErr   error
	captureErr   error
	releaseErr   error
}

func (*mobileVideoWorkerBillingFake) Apply(context.Context, *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	return &UsageBillingApplyResult{}, nil
}

func (f *mobileVideoWorkerBillingFake) ReserveBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	f.reserveCalls++
	return &BatchImageBalanceHoldResult{Applied: f.reserveErr == nil}, f.reserveErr
}

func (f *mobileVideoWorkerBillingFake) CaptureBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	f.captureCalls++
	return &BatchImageBalanceHoldResult{Applied: f.captureErr == nil}, f.captureErr
}

func (f *mobileVideoWorkerBillingFake) ReleaseBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	f.releaseCalls++
	return &BatchImageBalanceHoldResult{Applied: f.releaseErr == nil}, f.releaseErr
}

func fundedMobileVideoJobForTest(taskID string, state MobileVideoJobState) *MobileVideoJob {
	const (
		userID  int64 = 7
		groupID int64 = 9
		keyID   int64 = 17
	)
	groupIDCopy := groupID
	return &MobileVideoJob{
		TaskID: taskID, UserID: userID, GroupID: groupID, ExecutionAPIKeyID: keyID,
		ExecutionSnapshot: &MobileVideoExecutionSnapshot{
			Version: MobileVideoExecutionSnapshotVersion,
			APIKey: APIKeyAuthSnapshot{
				Version: 1, APIKeyID: keyID, UserID: userID, GroupID: &groupIDCopy,
				Name: "[managed:nextchat] Video Execution/test",
				User: APIKeyAuthUserSnapshot{ID: userID}, Group: &APIKeyAuthGroupSnapshot{ID: groupID},
			},
			EffectiveVideoRateMultiplier: 1,
		},
		UnitPriceUSD: 0.12, RateMultiplier: 1, HoldAmount: 0.96,
		Adapter: MobileVideoAdapterGrok, Model: "video-model", Prompt: "private prompt", Resolution: "720p", DurationSeconds: 8,
		State: state, BillingState: MobileVideoBillingStateReserved,
	}
}

func TestMobileVideoWorkerPersistsPrivateResultAndTransitionsTask(t *testing.T) {
	task, err := NewMobileTask("task-video-1", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-1", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	jobs := &mobileVideoWorkerJobStoreFake{job: &MobileVideoJob{
		TaskID: task.ID, UserID: 7, GroupID: 9, ExecutionAPIKeyID: 17,
		Adapter: MobileVideoAdapterGrok, Model: "video-model", Prompt: "private prompt",
		State: MobileVideoJobStateQueued,
	}}
	tasks := &mobileVideoWorkerTaskStoreFake{task: task}
	provider := &mobileVideoWorkerProviderFake{}
	worker := NewMobileVideoWorker(jobs, tasks, provider, mobileVideoWorkerStorageFake{}, MobileVideoWorkerOptions{WorkerID: "worker", PollInterval: time.Millisecond})

	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("first run processed=%v err=%v", processed, err)
	}
	if provider.creates != 1 || !jobs.submitted || tasks.task.Status != MobileTaskStatusRunning {
		t.Fatalf("create phase did not persist: creates=%d submitted=%v task=%s", provider.creates, jobs.submitted, tasks.task.Status)
	}

	processed, err = worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("second run processed=%v err=%v", processed, err)
	}
	if provider.polls != 1 || !tasks.finalized || tasks.task.Status != MobileTaskStatusCompleted {
		t.Fatalf("poll phase did not complete: polls=%d finalized=%v task=%s", provider.polls, tasks.finalized, tasks.task.Status)
	}
	if len(tasks.task.Artifacts) != 1 || tasks.task.Artifacts[0].URL != "/api/v1/mobile/video/jobs/task-video-1/content" {
		t.Fatalf("expected private content route artifact, got %+v", tasks.task.Artifacts)
	}
}

func TestMobileVideoWorkerFailsAmbiguousSubmissionWithoutResubmitting(t *testing.T) {
	task, err := NewMobileTask("task-video-unknown", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-unknown", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	jobs := &mobileVideoWorkerJobStoreFake{job: fundedMobileVideoJobForTest(task.ID, MobileVideoJobStateSubmissionUnknown)}
	tasks := &mobileVideoWorkerTaskStoreFake{task: task}
	provider := &mobileVideoWorkerProviderFake{}
	billing := &mobileVideoWorkerBillingFake{}
	worker := NewMobileVideoWorker(jobs, tasks, provider, mobileVideoWorkerStorageFake{}, MobileVideoWorkerOptions{WorkerID: "worker", Billing: billing})

	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("run processed=%v err=%v", processed, err)
	}
	if provider.creates != 0 || provider.polls != 0 {
		t.Fatalf("ambiguous submission must not call provider: creates=%d polls=%d", provider.creates, provider.polls)
	}
	if !jobs.failed || tasks.task.Status != MobileTaskStatusFailed || tasks.task.Error == nil || tasks.task.Error.Code != "VIDEO_SUBMISSION_UNKNOWN" {
		t.Fatalf("ambiguous submission was not made retryable terminal: jobs=%+v task=%+v", jobs, tasks.task)
	}
	if billing.captureCalls != 1 || !jobs.billingCaptured {
		t.Fatalf("ambiguous submission must capture its accepted hold exactly once: billing=%+v jobs=%+v", billing, jobs)
	}
}

func TestMobileVideoWorkerCancelsBeforeDispatchAndReleasesAcceptedHold(t *testing.T) {
	task, err := NewMobileTask("task-video-cancel-before-dispatch", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-cancel-before", time.Now().UTC())
	require.NoError(t, err)
	cancelled, err := CancelMobileTask(task, time.Now().UTC())
	require.NoError(t, err)
	jobs := &mobileVideoWorkerJobStoreFake{job: fundedMobileVideoJobForTest(task.ID, MobileVideoJobStateQueued)}
	tasks := &mobileVideoWorkerTaskStoreFake{task: cancelled}
	provider := &mobileVideoWorkerProviderFake{}
	billing := &mobileVideoWorkerBillingFake{}
	worker := NewMobileVideoWorker(jobs, tasks, provider, mobileVideoWorkerStorageFake{}, MobileVideoWorkerOptions{WorkerID: "worker", Billing: billing})

	processed, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Zero(t, provider.creates)
	require.Zero(t, provider.polls)
	require.Equal(t, 1, billing.releaseCalls)
	require.True(t, jobs.billingReleased)
	require.True(t, jobs.cancelled)
}

func TestMobileVideoWorkerCapturesLateCompletionAfterClientCancellationWithoutArtifact(t *testing.T) {
	task, err := NewMobileTask("task-video-cancel-after-submit", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-cancel-after", time.Now().UTC())
	require.NoError(t, err)
	cancelled, err := CancelMobileTask(task, time.Now().UTC())
	require.NoError(t, err)
	job := fundedMobileVideoJobForTest(task.ID, MobileVideoJobStatePolling)
	job.ProviderRequestID = "provider-1"
	jobs := &mobileVideoWorkerJobStoreFake{job: job, submitted: true}
	tasks := &mobileVideoWorkerTaskStoreFake{task: cancelled}
	provider := &mobileVideoWorkerProviderFake{}
	billing := &mobileVideoWorkerBillingFake{}
	worker := NewMobileVideoWorker(jobs, tasks, provider, mobileVideoWorkerStorageFake{}, MobileVideoWorkerOptions{WorkerID: "worker", Billing: billing})

	processed, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, provider.polls)
	require.Equal(t, 1, billing.captureCalls)
	require.Zero(t, billing.releaseCalls)
	require.True(t, jobs.billingCaptured)
	require.True(t, jobs.cancelled)
	require.False(t, tasks.finalized)
	require.Empty(t, tasks.task.Artifacts)
}

func TestMobileVideoWorkerReleasesProviderFailureAfterClientCancellation(t *testing.T) {
	task, err := NewMobileTask("task-video-provider-failure", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-provider-failure", time.Now().UTC())
	require.NoError(t, err)
	cancelled, err := CancelMobileTask(task, time.Now().UTC())
	require.NoError(t, err)
	job := fundedMobileVideoJobForTest(task.ID, MobileVideoJobStatePolling)
	job.ProviderRequestID = "provider-1"
	jobs := &mobileVideoWorkerJobStoreFake{job: job, submitted: true}
	tasks := &mobileVideoWorkerTaskStoreFake{task: cancelled}
	provider := &mobileVideoWorkerProviderFake{pollResult: MobileVideoProviderResult{Status: "failed", ErrorCode: "UPSTREAM_REJECTED", ErrorMessage: "rejected"}}
	billing := &mobileVideoWorkerBillingFake{}
	worker := NewMobileVideoWorker(jobs, tasks, provider, mobileVideoWorkerStorageFake{}, MobileVideoWorkerOptions{WorkerID: "worker", Billing: billing})

	processed, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, provider.polls)
	require.Zero(t, billing.captureCalls)
	require.Equal(t, 1, billing.releaseCalls)
	require.True(t, jobs.billingReleased)
	require.True(t, jobs.cancelled)
	require.Equal(t, MobileTaskStatusCancelled, tasks.task.Status)
}

func TestMobileVideoWorkerResumesLeasedReleaseWithoutProviderDispatch(t *testing.T) {
	for _, test := range []struct {
		name         string
		slug         string
		billingState MobileVideoBillingState
		wantReleases int
		cancelled    bool
	}{
		{name: "retries fenced release", slug: "releasing", billingState: MobileVideoBillingStateReleasing, wantReleases: 1},
		{name: "finishes acknowledged release", slug: "released", billingState: MobileVideoBillingStateReleased, wantReleases: 0},
		{name: "keeps cancellation terminal", slug: "cancelled", billingState: MobileVideoBillingStateReleasing, wantReleases: 1, cancelled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			task, err := NewMobileTask("task-video-release-"+test.slug, MobileTaskKindVideo, MobileVideoOperationGenerate, "client-release-"+test.slug, time.Now().UTC())
			require.NoError(t, err)
			if test.cancelled {
				task, err = CancelMobileTask(task, time.Now().UTC())
				require.NoError(t, err)
			}
			job := fundedMobileVideoJobForTest(task.ID, MobileVideoJobStatePolling)
			job.ProviderRequestID = "provider-1"
			job.BillingState = test.billingState
			jobs := &mobileVideoWorkerJobStoreFake{job: job, submitted: true}
			tasks := &mobileVideoWorkerTaskStoreFake{task: task}
			provider := &mobileVideoWorkerProviderFake{}
			billing := &mobileVideoWorkerBillingFake{}
			worker := NewMobileVideoWorker(jobs, tasks, provider, mobileVideoWorkerStorageFake{}, MobileVideoWorkerOptions{WorkerID: "worker", Billing: billing})

			processed, err := worker.RunOnce(context.Background())
			require.NoError(t, err)
			require.True(t, processed)
			require.Zero(t, provider.creates)
			require.Zero(t, provider.polls)
			require.Equal(t, test.wantReleases, billing.releaseCalls)
			require.True(t, jobs.billingReleased || test.billingState == MobileVideoBillingStateReleased)
			if test.cancelled {
				require.True(t, jobs.cancelled)
				require.False(t, jobs.failed)
				require.Equal(t, MobileTaskStatusCancelled, tasks.task.Status)
			} else {
				require.True(t, jobs.failed)
				require.Equal(t, MobileTaskStatusFailed, tasks.task.Status)
				require.NotNil(t, tasks.task.Error)
				require.Equal(t, "VIDEO_RELEASE_RECOVERED", tasks.task.Error.Code)
				require.True(t, tasks.task.Error.Retryable)
			}

			processed, err = worker.RunOnce(context.Background())
			require.NoError(t, err)
			require.False(t, processed)
			require.Equal(t, test.wantReleases, billing.releaseCalls)
			require.Zero(t, provider.creates)
			require.Zero(t, provider.polls)
		})
	}
}

func TestMobileVideoWorkerRecoversOnlyFencedFundingReleaseWithoutDispatching(t *testing.T) {
	task, err := NewMobileTask("task-video-releasing-recovery", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-releasing-recovery", time.Now().UTC())
	require.NoError(t, err)
	job := fundedMobileVideoJobForTest(task.ID, MobileVideoJobStateFunding)
	job.BillingState = MobileVideoBillingStateReleasing
	jobs := &mobileVideoWorkerJobStoreFake{job: job}
	tasks := &mobileVideoWorkerTaskStoreFake{task: task}
	provider := &mobileVideoWorkerProviderFake{}
	billing := &mobileVideoWorkerBillingFake{}
	worker := NewMobileVideoWorker(jobs, tasks, provider, mobileVideoWorkerStorageFake{}, MobileVideoWorkerOptions{WorkerID: "worker", Billing: billing})

	processed, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Zero(t, provider.creates)
	require.Zero(t, provider.polls)
	require.Equal(t, 1, billing.releaseCalls)
	require.True(t, jobs.discarded)

	processed, err = worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.False(t, processed, "completed compensation must not be claimed or released again")
	require.Equal(t, 1, billing.releaseCalls)
}

func TestBuildMobileVideoTaskDoesNotExposePromptOrReferenceIDs(t *testing.T) {
	input := MobileVideoJobCreateInput{
		GroupID: 1, ExecutionAPIKeyID: 2, Adapter: MobileVideoAdapterGrok,
		Model: "video-model", Prompt: "private prompt", Resolution: "720p", DurationSeconds: 8,
		ReferenceAssetIDs: []string{"asset-1"}, ClientRequestID: "request-1",
	}
	task, err := BuildMobileVideoTaskCreateInput(input)
	if err != nil {
		t.Fatal(err)
	}
	if task.Kind != MobileTaskKindVideo || task.Resource == nil || task.Resource.Type != "video_job" {
		t.Fatalf("unexpected public video task projection: %+v", task)
	}
	if task.Resource.ID == "private prompt" || task.Resource.ID == "asset-1" {
		t.Fatalf("private data leaked into task resource: %+v", task.Resource)
	}
}

func TestMobileVideoFundingRejectsNonFiniteAmounts(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := ValidateMobileVideoHoldAmount(value, 8, 1)
		require.ErrorIs(t, err, ErrMobileVideoFundingInvalid)
		_, err = ValidateMobileVideoHoldAmount(0.12, 8, value)
		require.ErrorIs(t, err, ErrMobileVideoFundingInvalid)
	}

	job := fundedMobileVideoJobForTest("task-video-non-finite", MobileVideoJobStateFunding)
	input := job.CreateInput()
	input.ClientRequestID = "funding-test"
	input.UnitPriceUSD = math.NaN()
	require.ErrorIs(t, ValidateMobileVideoJobCreateInput(input), ErrMobileVideoFundingInvalid)
}

func TestMobileVideoJobServiceActivatesOnlyAfterDurableReservation(t *testing.T) {
	taskService, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	_, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, deleted_at TIMESTAMP NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id) VALUES (7)`)
	require.NoError(t, err)

	input := fundedMobileVideoJobForTest("funding-template", MobileVideoJobStateFunding).CreateInput()
	input.ClientRequestID = "funding-activate"
	taskInput, err := BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)
	task, err := taskService.CreateVideoTask(context.Background(), 7, taskInput, input)
	require.NoError(t, err)

	store := NewMobileVideoJobService(db, "sqlite")
	before, err := store.Get(context.Background(), 7, task.ID)
	require.NoError(t, err)
	require.Equal(t, MobileVideoJobStateFunding, before.State)
	require.Equal(t, MobileVideoBillingStateFunding, before.BillingState)
	require.ErrorIs(t, store.ActivateReserved(context.Background(), 7, task.ID, 3), ErrMobileVideoFundingNotReady)

	require.NoError(t, store.MarkFundingReserved(context.Background(), task.ID))
	require.NoError(t, store.ActivateReserved(context.Background(), 7, task.ID, 3))
	after, err := store.Get(context.Background(), 7, task.ID)
	require.NoError(t, err)
	require.Equal(t, MobileVideoJobStateQueued, after.State)
	require.Equal(t, MobileVideoBillingStateReserved, after.BillingState)
}

func TestMobileVideoJobServiceEnforcesPerUserActivationCap(t *testing.T) {
	taskService, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	_, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, deleted_at TIMESTAMP NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id) VALUES (7)`)
	require.NoError(t, err)
	store := NewMobileVideoJobService(db, "sqlite")

	for index := 0; index < MobileVideoActiveJobsPerUser+1; index++ {
		input := fundedMobileVideoJobForTest("funding-template", MobileVideoJobStateFunding).CreateInput()
		input.ClientRequestID = fmt.Sprintf("funding-cap-%d", index)
		taskInput, err := BuildMobileVideoTaskCreateInput(input)
		require.NoError(t, err)
		task, err := taskService.CreateVideoTask(context.Background(), 7, taskInput, input)
		require.NoError(t, err)
		require.NoError(t, store.MarkFundingReserved(context.Background(), task.ID))
		err = store.ActivateReserved(context.Background(), 7, task.ID, MobileVideoActiveJobsPerUser)
		if index < MobileVideoActiveJobsPerUser {
			require.NoError(t, err)
			continue
		}
		require.ErrorIs(t, err, ErrMobileVideoQueueLimit)
	}
}

func TestMobileVideoJobServiceAllowsDiscardOnlyAfterNoHoldOrRecordedRelease(t *testing.T) {
	taskService, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	_, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, deleted_at TIMESTAMP NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id) VALUES (7)`)
	require.NoError(t, err)

	input := fundedMobileVideoJobForTest("funding-template", MobileVideoJobStateFunding).CreateInput()
	input.ClientRequestID = "funding-discard"
	taskInput, err := BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)
	task, err := taskService.CreateVideoTask(context.Background(), 7, taskInput, input)
	require.NoError(t, err)
	store := NewMobileVideoJobService(db, "sqlite")

	require.NoError(t, store.DiscardUnfunded(context.Background(), 7, task.ID))
	_, err = taskService.Get(context.Background(), 7, task.ID)
	require.ErrorIs(t, err, ErrMobileTaskNotFound)

	// A confirmed hold must be released and recorded before its public task can
	// be discarded. The store rejects deletion while the reservation is live.
	input.ClientRequestID = "funding-release-before-discard"
	taskInput, err = BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)
	task, err = taskService.CreateVideoTask(context.Background(), 7, taskInput, input)
	require.NoError(t, err)
	require.NoError(t, store.MarkFundingReserved(context.Background(), task.ID))
	require.ErrorIs(t, store.DiscardUnfunded(context.Background(), 7, task.ID), ErrMobileVideoFundingNotReady)
	require.NoError(t, store.MarkFundingReleasePending(context.Background(), task.ID))
	require.ErrorIs(t, store.ActivateReserved(context.Background(), 7, task.ID, MobileVideoActiveJobsPerUser), ErrMobileVideoFundingNotReady)
	require.NoError(t, store.MarkFundingReleased(context.Background(), task.ID))
	require.NoError(t, store.DiscardUnfunded(context.Background(), 7, task.ID))
}

func TestMobileVideoJobServiceClaimsOnlyFencedFundingRelease(t *testing.T) {
	taskService, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	_, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, deleted_at TIMESTAMP NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id) VALUES (7)`)
	require.NoError(t, err)
	store := NewMobileVideoJobService(db, "sqlite")

	create := func(requestID string) *MobileTask {
		input := fundedMobileVideoJobForTest("funding-template", MobileVideoJobStateFunding).CreateInput()
		input.ClientRequestID = requestID
		taskInput, createErr := BuildMobileVideoTaskCreateInput(input)
		require.NoError(t, createErr)
		task, createErr := taskService.CreateVideoTask(context.Background(), 7, taskInput, input)
		require.NoError(t, createErr)
		return task
	}

	ordinary := create("ordinary-funding")
	claimed, err := store.ClaimFundingRelease(context.Background(), "worker", time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.Nil(t, claimed, "ordinary funding must never be claimed by recovery")

	fenced := create("fenced-release")
	require.NoError(t, store.MarkFundingReserved(context.Background(), fenced.ID))
	require.NoError(t, store.MarkFundingReleasePending(context.Background(), fenced.ID))
	require.ErrorIs(t, store.ActivateReserved(context.Background(), 7, fenced.ID, MobileVideoActiveJobsPerUser), ErrMobileVideoFundingNotReady)

	claimed, err = store.ClaimFundingRelease(context.Background(), "worker", time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.Equal(t, fenced.ID, claimed.TaskID)
	require.Equal(t, MobileVideoJobStateFunding, claimed.State)
	require.Equal(t, MobileVideoBillingStateReleasing, claimed.BillingState)
	require.Equal(t, "worker", claimed.LeaseOwner)

	again, err := store.ClaimFundingRelease(context.Background(), "another-worker", time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.Nil(t, again, "the active recovery lease prevents duplicate release attempts")

	require.NoError(t, store.MarkFundingReleased(context.Background(), fenced.ID))
	require.NoError(t, store.DiscardUnfunded(context.Background(), 7, fenced.ID))
	remaining, err := store.Get(context.Background(), 7, ordinary.ID)
	require.NoError(t, err)
	require.Equal(t, MobileVideoBillingStateFunding, remaining.BillingState)
}

func TestMobileVideoJobServicePreventsOpposingBillingSettlementAfterTerminalState(t *testing.T) {
	taskService, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	_, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, deleted_at TIMESTAMP NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id) VALUES (7)`)
	require.NoError(t, err)
	store := NewMobileVideoJobService(db, "sqlite")

	createPollingReserved := func(requestID string) *MobileTask {
		input := fundedMobileVideoJobForTest("funding-template", MobileVideoJobStateFunding).CreateInput()
		input.ClientRequestID = requestID
		taskInput, createErr := BuildMobileVideoTaskCreateInput(input)
		require.NoError(t, createErr)
		task, createErr := taskService.CreateVideoTask(context.Background(), 7, taskInput, input)
		require.NoError(t, createErr)
		_, createErr = db.Exec(`UPDATE mobile_video_jobs
			SET state = 'polling', billing_state = 'reserved', lease_owner = 'worker'
			WHERE task_id = ?`, task.ID)
		require.NoError(t, createErr)
		return task
	}

	released := createPollingReserved("funding-release-terminal")
	require.NoError(t, store.MarkBillingReleasePending(context.Background(), released.ID, "worker"))
	require.NoError(t, store.MarkBillingReleased(context.Background(), released.ID, "worker"))
	require.ErrorIs(t, store.MarkBillingCaptured(context.Background(), released.ID, "worker"), ErrMobileVideoJobNotFound)
	releasedJob, err := store.Get(context.Background(), 7, released.ID)
	require.NoError(t, err)
	require.Equal(t, MobileVideoBillingStateReleased, releasedJob.BillingState)

	captured := createPollingReserved("funding-capture-terminal")
	require.NoError(t, store.MarkBillingCaptured(context.Background(), captured.ID, "worker"))
	require.ErrorIs(t, store.MarkBillingReleased(context.Background(), captured.ID, "worker"), ErrMobileVideoJobNotFound)
	capturedJob, err := store.Get(context.Background(), 7, captured.ID)
	require.NoError(t, err)
	require.Equal(t, MobileVideoBillingStateCaptured, capturedJob.BillingState)
}
