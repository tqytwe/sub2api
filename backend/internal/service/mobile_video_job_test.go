package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mobileVideoWorkerTaskStoreFake struct {
	task        MobileTask
	transitions []MobileTaskTransitionInput
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
		s.task.Artifacts = input.Artifacts
	}
	s.task.Error = input.Error
	return &s.task, nil
}

type mobileVideoWorkerJobStoreFake struct {
	job          *MobileVideoJob
	submitted    bool
	completed    bool
	failed       bool
	cancelled    bool
	heartbeats   int
	heartbeatErr error
	heartbeatHit chan struct{}
	storageKey   string
	artifactURL  string
	artifactType string
	artifactSize int64
}

func (s *mobileVideoWorkerJobStoreFake) Create(context.Context, int64, string, MobileVideoJobCreateInput, string) error {
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
	copy := *s.job
	copy.LeaseOwner = "worker"
	if s.submitted {
		copy.ProviderRequestID = "provider-1"
	}
	return &copy, nil
}
func (s *mobileVideoWorkerJobStoreFake) Heartbeat(context.Context, string, string, time.Time, time.Duration) error {
	s.heartbeats++
	if s.heartbeatHit != nil {
		select {
		case s.heartbeatHit <- struct{}{}:
		default:
		}
	}
	return s.heartbeatErr
}
func (s *mobileVideoWorkerJobStoreFake) MarkSubmitted(_ context.Context, _ string, _ string, providerRequestID string, _ time.Time) error {
	s.submitted = true
	s.job.ProviderRequestID = providerRequestID
	return nil
}
func (s *mobileVideoWorkerJobStoreFake) MarkCompleted(_ context.Context, _ string, _ string, storageKey, artifactURL, contentType string, byteSize int64) error {
	s.completed = true
	s.storageKey, s.artifactURL, s.artifactType, s.artifactSize = storageKey, artifactURL, contentType, byteSize
	return nil
}
func (s *mobileVideoWorkerJobStoreFake) MarkFailed(context.Context, string, string, string, string, bool) error {
	s.failed = true
	return nil
}
func (s *mobileVideoWorkerJobStoreFake) MarkCancelled(context.Context, string, string) error {
	s.cancelled = true
	return nil
}

type mobileVideoWorkerProviderFake struct {
	creates int
	polls   int
}

func (p *mobileVideoWorkerProviderFake) Create(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	p.creates++
	return MobileVideoProviderResult{RequestID: "provider-1", Status: "queued", NextPollAfter: time.Millisecond}, nil
}
func (p *mobileVideoWorkerProviderFake) Poll(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	p.polls++
	return MobileVideoProviderResult{RequestID: "provider-1", Status: "completed", Artifact: []byte("video-bytes"), ContentType: "video/mp4"}, nil
}
func (p *mobileVideoWorkerProviderFake) Content(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	return MobileVideoProviderResult{}, errors.New("content should not be requested when poll includes bytes")
}

type mobileVideoWorkerStorageFake struct{}

func (mobileVideoWorkerStorageFake) Save(context.Context, string, string, []byte) (string, error) {
	return "https://storage.example/video.mp4", nil
}

func TestMobileVideoWorkerPersistsProviderResultAndTransitionsTask(t *testing.T) {
	createdAt := time.Now().UTC()
	task, err := NewMobileTask("task-video-1", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-1", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	jobStore := &mobileVideoWorkerJobStoreFake{job: &MobileVideoJob{TaskID: task.ID, UserID: 7, GroupID: 9, Model: "video-model", Prompt: "waves", State: MobileVideoJobStateQueued}}
	tasks := &mobileVideoWorkerTaskStoreFake{task: task}
	provider := &mobileVideoWorkerProviderFake{}
	worker := NewMobileVideoWorker(jobStore, tasks, provider, mobileVideoWorkerStorageFake{}, MobileVideoWorkerOptions{WorkerID: "worker", PollInterval: time.Millisecond})

	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("first run processed=%v err=%v", processed, err)
	}
	if provider.creates != 1 || !jobStore.submitted || tasks.task.Status != MobileTaskStatusRunning {
		t.Fatalf("create phase did not persist: creates=%d submitted=%v task=%s", provider.creates, jobStore.submitted, tasks.task.Status)
	}

	processed, err = worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("second run processed=%v err=%v", processed, err)
	}
	if provider.polls != 1 || !jobStore.completed || tasks.task.Status != MobileTaskStatusCompleted {
		t.Fatalf("poll phase did not complete: polls=%d completed=%v task=%s", provider.polls, jobStore.completed, tasks.task.Status)
	}
	if len(tasks.task.Artifacts) != 1 || tasks.task.Artifacts[0].ContentType != "video/mp4" || tasks.task.Artifacts[0].ByteSize != int64(len("video-bytes")) {
		t.Fatalf("artifact projection not persisted: %+v", tasks.task.Artifacts)
	}
}

func TestMobileVideoWorkerFailsProviderErrorWithoutLeakingPrompt(t *testing.T) {
	createdAt := time.Now().UTC()
	task, err := NewMobileTask("task-video-2", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-2", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	jobStore := &mobileVideoWorkerJobStoreFake{job: &MobileVideoJob{TaskID: task.ID, UserID: 7, GroupID: 9, Model: "video-model", Prompt: "secret prompt", State: MobileVideoJobStateQueued}}
	tasks := &mobileVideoWorkerTaskStoreFake{task: task}
	provider := mobileVideoProviderErrorFake{}
	worker := NewMobileVideoWorker(jobStore, tasks, provider, nil, MobileVideoWorkerOptions{WorkerID: "worker"})
	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed || !jobStore.failed || tasks.task.Status != MobileTaskStatusFailed {
		t.Fatalf("provider failure not persisted: processed=%v err=%v failed=%v task=%s", processed, err, jobStore.failed, tasks.task.Status)
	}
	if len(tasks.task.Error.Details) != 0 && containsSensitiveMobileVideoTestValue(tasks.task.Error.Details, "secret prompt") {
		t.Fatal("provider error exposed private prompt")
	}
}

func TestMobileVideoWorkerFailsRecoveredUnknownSubmissionWithoutCreatingAgain(t *testing.T) {
	createdAt := time.Now().UTC()
	task, err := NewMobileTask("task-video-unknown", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-unknown", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	jobStore := &mobileVideoWorkerJobStoreFake{job: &MobileVideoJob{
		TaskID: task.ID, UserID: 7, GroupID: 9, Model: "video-model", Prompt: "waves",
		State: MobileVideoJobStateSubmissionUnknown,
	}}
	tasks := &mobileVideoWorkerTaskStoreFake{task: task}
	provider := &mobileVideoWorkerProviderFake{}
	worker := NewMobileVideoWorker(jobStore, tasks, provider, nil, MobileVideoWorkerOptions{WorkerID: "worker"})

	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("unknown submission processed=%v err=%v", processed, err)
	}
	if provider.creates != 0 || provider.polls != 0 {
		t.Fatalf("unknown submission must not call the provider again: creates=%d polls=%d", provider.creates, provider.polls)
	}
	if !jobStore.failed || tasks.task.Status != MobileTaskStatusFailed {
		t.Fatalf("unknown submission must be surfaced as retryable failure: failed=%v task=%s", jobStore.failed, tasks.task.Status)
	}
}

type mobileVideoBlockingProviderFake struct {
	started chan struct{}
	release chan struct{}
}

func (p *mobileVideoBlockingProviderFake) Create(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	close(p.started)
	<-p.release
	return MobileVideoProviderResult{RequestID: "provider-1", Status: "queued"}, nil
}

func (p *mobileVideoBlockingProviderFake) Poll(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	return MobileVideoProviderResult{}, errors.New("unexpected poll")
}

func (p *mobileVideoBlockingProviderFake) Content(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	return MobileVideoProviderResult{}, errors.New("unexpected content")
}

func TestMobileVideoWorkerHeartbeatsWhileProviderCallIsInFlight(t *testing.T) {
	createdAt := time.Now().UTC()
	task, err := NewMobileTask("task-video-heartbeat", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-heartbeat", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	jobStore := &mobileVideoWorkerJobStoreFake{job: &MobileVideoJob{TaskID: task.ID, UserID: 7, GroupID: 9, Model: "video-model", Prompt: "waves", State: MobileVideoJobStateQueued}, heartbeatHit: make(chan struct{}, 1)}
	tasks := &mobileVideoWorkerTaskStoreFake{task: task}
	provider := &mobileVideoBlockingProviderFake{started: make(chan struct{}), release: make(chan struct{})}
	worker := NewMobileVideoWorker(jobStore, tasks, provider, nil, MobileVideoWorkerOptions{WorkerID: "worker", Lease: 30 * time.Millisecond})

	done := make(chan error, 1)
	go func() {
		_, runErr := worker.RunOnce(context.Background())
		done <- runErr
	}()
	<-provider.started
	select {
	case <-jobStore.heartbeatHit:
	case <-time.After(time.Second):
		t.Fatal("worker did not heartbeat while provider call was in flight")
	}
	close(provider.release)
	if err := <-done; err != nil {
		t.Fatalf("worker returned %v", err)
	}
	if !jobStore.submitted {
		t.Fatal("worker did not persist the provider request after the call")
	}
}

func TestMobileVideoWorkerDoesNotPersistProviderResultAfterLeaseLoss(t *testing.T) {
	createdAt := time.Now().UTC()
	task, err := NewMobileTask("task-video-heartbeat-loss", MobileTaskKindVideo, MobileVideoOperationGenerate, "client-heartbeat-loss", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	jobStore := &mobileVideoWorkerJobStoreFake{
		job:          &MobileVideoJob{TaskID: task.ID, UserID: 7, GroupID: 9, Model: "video-model", Prompt: "waves", State: MobileVideoJobStateQueued},
		heartbeatErr: errors.New("lease row disappeared"),
		heartbeatHit: make(chan struct{}, 1),
	}
	tasks := &mobileVideoWorkerTaskStoreFake{task: task}
	provider := &mobileVideoBlockingProviderFake{started: make(chan struct{}), release: make(chan struct{})}
	worker := NewMobileVideoWorker(
		jobStore,
		tasks,
		provider,
		nil,
		MobileVideoWorkerOptions{WorkerID: "worker", Lease: 30 * time.Millisecond},
	)

	done := make(chan error, 1)
	go func() {
		_, runErr := worker.RunOnce(context.Background())
		done <- runErr
	}()
	<-provider.started
	select {
	case <-jobStore.heartbeatHit:
	case <-time.After(time.Second):
		t.Fatal("worker did not report the failed heartbeat")
	}
	// The provider may ignore cancellation; let it return a successful result
	// after the lease has already been lost and verify it is discarded.
	close(provider.release)
	if err := <-done; err != nil {
		t.Fatalf("worker returned %v", err)
	}
	if jobStore.submitted || jobStore.completed || tasks.task.Status == MobileTaskStatusCompleted {
		t.Fatalf("provider result was persisted after lease loss: submitted=%v completed=%v task=%s", jobStore.submitted, jobStore.completed, tasks.task.Status)
	}
}

type mobileVideoProviderErrorFake struct{}

func (mobileVideoProviderErrorFake) Create(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	return MobileVideoProviderResult{}, &MobileVideoProviderError{Code: "UPSTREAM_TIMEOUT", Message: "upstream timed out", Retryable: true}
}
func (mobileVideoProviderErrorFake) Poll(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	return MobileVideoProviderResult{}, nil
}
func (mobileVideoProviderErrorFake) Content(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error) {
	return MobileVideoProviderResult{}, nil
}

func containsSensitiveMobileVideoTestValue(values map[string]any, target string) bool {
	for _, value := range values {
		if text, ok := value.(string); ok && text == target {
			return true
		}
	}
	return false
}
