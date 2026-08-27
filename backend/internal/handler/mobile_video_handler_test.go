package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mobileVideoHandlerTasksFake struct {
	created        []service.MobileTaskCreateInput
	videoInputs    []service.MobileVideoJobCreateInput
	videoJobs      *mobileVideoHandlerJobsFake
	createVideoErr error
	retryVideoErr  error
	tasks          map[string]*service.MobileTask
	cancelCalls    []string
	retryCalls     []string
}

func (f *mobileVideoHandlerTasksFake) Create(_ context.Context, _ int64, input service.MobileTaskCreateInput) (*service.MobileTask, error) {
	f.created = append(f.created, input)
	task, err := service.NewMobileTask("00000000-0000-0000-0000-000000000001", input.Kind, input.Operation, input.ClientRequestID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	task.Resource = input.Resource
	if f.tasks == nil {
		f.tasks = map[string]*service.MobileTask{}
	}
	f.tasks[task.ID] = &task
	return &task, nil
}

func (f *mobileVideoHandlerTasksFake) CreateVideoTask(ctx context.Context, userID int64, taskInput service.MobileTaskCreateInput, videoInput service.MobileVideoJobCreateInput) (*service.MobileTask, error) {
	f.videoInputs = append(f.videoInputs, videoInput)
	if f.createVideoErr != nil {
		return nil, f.createVideoErr
	}
	task, err := f.Create(ctx, userID, taskInput)
	if err == nil && task != nil && f.videoJobs != nil {
		err = f.videoJobs.Create(ctx, userID, task.ID, videoInput)
	}
	return task, err
}

func (f *mobileVideoHandlerTasksFake) List(context.Context, int64, service.MobileTaskListFilter) (*service.MobileTaskPage, error) {
	return &service.MobileTaskPage{}, nil
}

func (f *mobileVideoHandlerTasksFake) Get(_ context.Context, _ int64, id string) (*service.MobileTask, error) {
	if task := f.tasks[id]; task != nil {
		copy := *task
		return &copy, nil
	}
	return nil, service.ErrMobileTaskNotFound
}

func (f *mobileVideoHandlerTasksFake) Cancel(ctx context.Context, userID int64, id string) (*service.MobileTask, error) {
	f.cancelCalls = append(f.cancelCalls, id)
	return f.Get(ctx, userID, id)
}

func (f *mobileVideoHandlerTasksFake) Retry(ctx context.Context, userID int64, id, clientRequestID string) (*service.MobileTask, error) {
	f.retryCalls = append(f.retryCalls, id)
	task, err := f.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return f.Create(ctx, userID, service.MobileTaskCreateInput{Kind: task.Kind, Operation: task.Operation, ClientRequestID: clientRequestID, Resource: task.Resource})
}

func (f *mobileVideoHandlerTasksFake) RetryVideoTask(ctx context.Context, userID int64, id, clientRequestID string, videoInput service.MobileVideoJobCreateInput) (*service.MobileTask, error) {
	f.retryCalls = append(f.retryCalls, id)
	f.videoInputs = append(f.videoInputs, videoInput)
	if f.retryVideoErr != nil {
		return nil, f.retryVideoErr
	}
	task, err := f.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	retry, err := f.Create(ctx, userID, service.MobileTaskCreateInput{
		Kind: task.Kind, Operation: task.Operation, ClientRequestID: clientRequestID,
		ParentTaskID: task.ID, Resource: task.Resource,
	})
	if err == nil && retry != nil && f.videoJobs != nil {
		err = f.videoJobs.Create(ctx, userID, retry.ID, videoInput)
	}
	return retry, err
}

func (f *mobileVideoHandlerTasksFake) Transition(context.Context, int64, string, service.MobileTaskTransitionInput) (*service.MobileTask, error) {
	return nil, nil
}

type mobileVideoHandlerResolverFake struct {
	bootstrap *service.MobileVideoBootstrap
	resolved  *service.MobileVideoResolvedModel
	err       error
	requests  []struct {
		userID  int64
		groupID int64
		model   string
	}
}

func (f *mobileVideoHandlerResolverFake) Bootstrap(context.Context, int64) (*service.MobileVideoBootstrap, error) {
	return f.bootstrap, f.err
}

func (f *mobileVideoHandlerResolverFake) Resolve(_ context.Context, userID, groupID int64, model string) (*service.MobileVideoResolvedModel, error) {
	f.requests = append(f.requests, struct {
		userID  int64
		groupID int64
		model   string
	}{userID: userID, groupID: groupID, model: model})
	return f.resolved, f.err
}

type mobileVideoHandlerExecutionFake struct {
	session       *service.NextChatManagedSession
	snapshot      *service.MobileVideoExecutionSnapshot
	snapshotErr   error
	calls         []struct{ userID, groupID int64 }
	snapshotCalls []struct{ userID, groupID, keyID int64 }
}

func (f *mobileVideoHandlerExecutionFake) IssueMobileVideoExecutionSession(_ context.Context, userID, groupID int64) (*service.NextChatManagedSession, error) {
	f.calls = append(f.calls, struct{ userID, groupID int64 }{userID: userID, groupID: groupID})
	return f.session, nil
}

func (f *mobileVideoHandlerExecutionFake) CaptureMobileVideoExecutionSnapshot(_ context.Context, userID, groupID, keyID int64) (*service.MobileVideoExecutionSnapshot, error) {
	f.snapshotCalls = append(f.snapshotCalls, struct{ userID, groupID, keyID int64 }{userID, groupID, keyID})
	if f.snapshotErr != nil {
		return nil, f.snapshotErr
	}
	if f.snapshot != nil {
		copy := *f.snapshot
		return &copy, nil
	}
	return mobileVideoHandlerExecutionSnapshot(userID, groupID, keyID, 1), nil
}

func mobileVideoHandlerExecutionSnapshot(userID, groupID, keyID int64, multiplier float64) *service.MobileVideoExecutionSnapshot {
	groupIDCopy := groupID
	return &service.MobileVideoExecutionSnapshot{
		Version: service.MobileVideoExecutionSnapshotVersion,
		APIKey: service.APIKeyAuthSnapshot{
			Version:  1,
			APIKeyID: keyID,
			UserID:   userID,
			GroupID:  &groupIDCopy,
			Name:     "[managed:nextchat] Video Execution/test",
			User:     service.APIKeyAuthUserSnapshot{ID: userID},
			Group:    &service.APIKeyAuthGroupSnapshot{ID: groupID},
		},
		EffectiveVideoRateMultiplier: multiplier,
	}
}

type mobileVideoHandlerJobsFake struct {
	created                 []service.MobileVideoJobCreateInput
	jobs                    map[string]*service.MobileVideoJob
	releaseCalls            []string
	releaseErr              error
	markFundingReservedErr  error
	markFundingReleaseErr   error
	markFundingReleasedErr  error
	activateErr             error
	discardErr              error
	clientCancellationCalls []string
	markCancelledCalls      []string
	activateCalls           []string
	discardCalls            []string
	fundingReleaseCalls     []string
}

func (f *mobileVideoHandlerJobsFake) Create(_ context.Context, _ int64, taskID string, input service.MobileVideoJobCreateInput) error {
	f.created = append(f.created, input)
	if f.jobs == nil {
		f.jobs = map[string]*service.MobileVideoJob{}
	}
	if _, exists := f.jobs[taskID]; exists {
		return nil
	}
	f.jobs[taskID] = &service.MobileVideoJob{
		TaskID: taskID, UserID: 42, GroupID: input.GroupID, ExecutionAPIKeyID: input.ExecutionAPIKeyID,
		ExecutionSnapshot: input.ExecutionSnapshot, UnitPriceUSD: input.UnitPriceUSD, RateMultiplier: input.RateMultiplier,
		HoldAmount: input.HoldAmount, Adapter: input.Adapter, Model: input.Model, Prompt: input.Prompt,
		Resolution: input.Resolution, Ratio: input.Ratio, DurationSeconds: input.DurationSeconds,
		State: service.MobileVideoJobStateFunding, BillingState: service.MobileVideoBillingStateFunding,
	}
	return nil
}

func (f *mobileVideoHandlerJobsFake) Get(_ context.Context, _ int64, taskID string) (*service.MobileVideoJob, error) {
	if job := f.jobs[taskID]; job != nil {
		copy := *job
		return &copy, nil
	}
	// Handler tests use a lightweight public-task fake that does not share the
	// real task-service transaction. Model the newly persisted private row here
	// so the test still verifies the handler's funding sequence.
	if f.jobs == nil {
		f.jobs = map[string]*service.MobileVideoJob{}
	}
	f.jobs[taskID] = &service.MobileVideoJob{
		TaskID: taskID, UserID: 42, GroupID: 9, ExecutionAPIKeyID: 99,
		ExecutionSnapshot: mobileVideoHandlerExecutionSnapshot(42, 9, 99, 1),
		UnitPriceUSD:      0.12, RateMultiplier: 1, HoldAmount: 0.96,
		Adapter: service.MobileVideoAdapterGrok, Model: "video-alpha", Prompt: "test", Resolution: "720p", Ratio: "16:9", DurationSeconds: 8,
		State: service.MobileVideoJobStateFunding, BillingState: service.MobileVideoBillingStateFunding,
	}
	copy := *f.jobs[taskID]
	return &copy, nil
}

func (*mobileVideoHandlerJobsFake) ClaimNext(context.Context, string, time.Time, time.Duration) (*service.MobileVideoJob, error) {
	return nil, service.ErrMobileVideoJobNotFound
}
func (*mobileVideoHandlerJobsFake) ClaimFundingRelease(context.Context, string, time.Time, time.Duration) (*service.MobileVideoJob, error) {
	return nil, service.ErrMobileVideoJobNotFound
}
func (*mobileVideoHandlerJobsFake) Heartbeat(context.Context, string, string, time.Time, time.Duration) error {
	return nil
}
func (*mobileVideoHandlerJobsFake) MarkSubmitted(context.Context, string, string, string, time.Time) error {
	return nil
}
func (*mobileVideoHandlerJobsFake) MarkSubmissionUnknown(context.Context, string, string, string) error {
	return nil
}
func (*mobileVideoHandlerJobsFake) MarkCompleted(context.Context, string, string, string, string, int64) error {
	return nil
}
func (*mobileVideoHandlerJobsFake) MarkFailed(context.Context, string, string, string, string, bool) error {
	return nil
}
func (f *mobileVideoHandlerJobsFake) MarkClientCancelled(_ context.Context, taskID string) error {
	f.clientCancellationCalls = append(f.clientCancellationCalls, taskID)
	if job := f.jobs[taskID]; job != nil {
		now := time.Now().UTC()
		job.ClientCancelledAt = &now
	}
	return nil
}
func (f *mobileVideoHandlerJobsFake) MarkFundingReserved(_ context.Context, taskID string) error {
	if f.markFundingReservedErr != nil {
		return f.markFundingReservedErr
	}
	if job := f.jobs[taskID]; job != nil {
		job.BillingState = service.MobileVideoBillingStateReserved
	}
	return nil
}
func (f *mobileVideoHandlerJobsFake) MarkFundingReleasePending(_ context.Context, taskID string) error {
	f.fundingReleaseCalls = append(f.fundingReleaseCalls, taskID)
	if f.markFundingReleaseErr != nil {
		return f.markFundingReleaseErr
	}
	if job := f.jobs[taskID]; job != nil {
		job.BillingState = service.MobileVideoBillingStateReleasing
	}
	return nil
}
func (f *mobileVideoHandlerJobsFake) MarkFundingReleased(_ context.Context, taskID string) error {
	if f.markFundingReleasedErr != nil {
		return f.markFundingReleasedErr
	}
	if job := f.jobs[taskID]; job != nil {
		job.BillingState = service.MobileVideoBillingStateReleased
	}
	return nil
}
func (f *mobileVideoHandlerJobsFake) MarkCancelled(_ context.Context, taskID, _ string) error {
	f.markCancelledCalls = append(f.markCancelledCalls, taskID)
	if job := f.jobs[taskID]; job != nil {
		job.State = service.MobileVideoJobStateCancelled
	}
	return nil
}
func (*mobileVideoHandlerJobsFake) MarkBillingCaptured(context.Context, string, string) error {
	return nil
}
func (*mobileVideoHandlerJobsFake) MarkBillingReleasePending(context.Context, string, string) error {
	return nil
}
func (*mobileVideoHandlerJobsFake) MarkBillingReleased(context.Context, string, string) error {
	return nil
}
func (f *mobileVideoHandlerJobsFake) ActivateReserved(_ context.Context, _ int64, taskID string, _ int) error {
	f.activateCalls = append(f.activateCalls, taskID)
	if f.activateErr != nil {
		return f.activateErr
	}
	if job := f.jobs[taskID]; job != nil {
		job.State = service.MobileVideoJobStateQueued
	}
	return nil
}
func (f *mobileVideoHandlerJobsFake) DiscardUnfunded(_ context.Context, _ int64, taskID string) error {
	f.discardCalls = append(f.discardCalls, taskID)
	if f.discardErr != nil {
		return f.discardErr
	}
	delete(f.jobs, taskID)
	return nil
}
func (f *mobileVideoHandlerJobsFake) ReleaseArtifact(_ context.Context, _ int64, taskID string) error {
	f.releaseCalls = append(f.releaseCalls, taskID)
	if f.releaseErr != nil {
		return f.releaseErr
	}
	if job := f.jobs[taskID]; job != nil {
		job.ArtifactStorageKey = ""
		job.ArtifactContentType = ""
		job.ArtifactByteSize = 0
	}
	return nil
}

type mobileVideoHandlerStorageFake struct{}

type mobileVideoHandlerBillingFake struct {
	reserveErr error
	captureErr error
	releaseErr error
	reserves   []*service.BatchImageBalanceHoldCommand
	captures   []*service.BatchImageBalanceHoldCommand
	releases   []*service.BatchImageBalanceHoldCommand
}

func (*mobileVideoHandlerBillingFake) Apply(context.Context, *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	return &service.UsageBillingApplyResult{}, nil
}

func (f *mobileVideoHandlerBillingFake) ReserveBatchImageBalance(_ context.Context, command *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	f.reserves = append(f.reserves, command)
	return &service.BatchImageBalanceHoldResult{Applied: f.reserveErr == nil}, f.reserveErr
}

func (f *mobileVideoHandlerBillingFake) CaptureBatchImageBalance(_ context.Context, command *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	f.captures = append(f.captures, command)
	return &service.BatchImageBalanceHoldResult{Applied: f.captureErr == nil}, f.captureErr
}

func (f *mobileVideoHandlerBillingFake) ReleaseBatchImageBalance(_ context.Context, command *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	f.releases = append(f.releases, command)
	return &service.BatchImageBalanceHoldResult{Applied: f.releaseErr == nil}, f.releaseErr
}

func (mobileVideoHandlerStorageFake) Save(context.Context, string, string, []byte) (string, error) {
	return "private/video.mp4", nil
}
func (mobileVideoHandlerStorageFake) Open(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader([]byte("video"))), "video/mp4", nil
}
func (mobileVideoHandlerStorageFake) Delete(context.Context, string) error { return nil }

type mobileVideoHandlerAcknowledgementStorageFake struct {
	deleteCalls  []string
	deleteErr    error
	beforeDelete func()
}

func (*mobileVideoHandlerAcknowledgementStorageFake) Save(context.Context, string, string, []byte) (string, error) {
	return "private/video.mp4", nil
}

func (*mobileVideoHandlerAcknowledgementStorageFake) Open(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader([]byte("video"))), "video/mp4", nil
}

func (f *mobileVideoHandlerAcknowledgementStorageFake) Delete(_ context.Context, key string) error {
	if f.beforeDelete != nil {
		f.beforeDelete()
	}
	f.deleteCalls = append(f.deleteCalls, key)
	return f.deleteErr
}

func mobileVideoHandlerResolvedModel() *service.MobileVideoResolvedModel {
	return &service.MobileVideoResolvedModel{
		ID: "video-alpha", Name: "Video Alpha", Model: "video-alpha",
		Modalities: []string{"video"}, Platform: service.PlatformGrok, CapabilityVersion: service.MobileVideoCapabilitiesVersion,
		GroupID: 9, Adapter: service.MobileVideoAdapterGrok,
		Capabilities: service.MobileVideoCapabilities{
			Operations: []string{"generate"}, SupportedResolutions: []string{"720p"},
			SupportedRatios: []string{"16:9"}, SupportedDurations: []int{8},
		},
		UnitPrices: map[string]float64{"720p": 0.12},
	}
}

func TestMobileVideoHandlerRequiresExplicitVideoPurposeBeforeResolvingOrCreating(t *testing.T) {
	tasks := &mobileVideoHandlerTasksFake{}
	resolver := &mobileVideoHandlerResolverFake{resolved: mobileVideoHandlerResolvedModel()}
	execution := &mobileVideoHandlerExecutionFake{session: &service.NextChatManagedSession{UserID: 42, KeyID: 99, APIKey: "video-only", Purpose: service.NextChatSessionPurposeVideo}}
	jobs := &mobileVideoHandlerJobsFake{}
	h := newMobileVideoHandlerWithDependencies(tasks, resolver, execution, jobs, mobileVideoHandlerStorageFake{}, &mobileVideoHandlerBillingFake{})

	missingPurpose := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", `{"group_id":9,"model":"video-alpha","prompt":"test","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"r1"}`)
	require.Equal(t, http.StatusBadRequest, missingPurpose.Code, missingPurpose.Body.String())
	require.Empty(t, resolver.requests)
	require.Empty(t, execution.calls)
	require.Empty(t, tasks.created)

	created := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", `{"purpose":"video","group_id":9,"model":"video-alpha","prompt":"test","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"r2"}`)
	require.Equal(t, http.StatusAccepted, created.Code, created.Body.String())
	require.Len(t, resolver.requests, 1)
	require.Equal(t, int64(42), resolver.requests[0].userID)
	require.Equal(t, int64(9), resolver.requests[0].groupID)
	require.Equal(t, "video-alpha", resolver.requests[0].model)
	require.Equal(t, []struct{ userID, groupID int64 }{{42, 9}}, execution.calls)
	require.Len(t, tasks.created, 1)
	require.Equal(t, service.MobileTaskKindVideo, tasks.created[0].Kind)
	require.Len(t, tasks.videoInputs, 1)
	require.Equal(t, int64(99), tasks.videoInputs[0].ExecutionAPIKeyID)
}

func TestMobileVideoHandlerRejectsWrongOrUnlabelledManagedSession(t *testing.T) {
	tasks := &mobileVideoHandlerTasksFake{}
	resolver := &mobileVideoHandlerResolverFake{resolved: mobileVideoHandlerResolvedModel()}
	execution := &mobileVideoHandlerExecutionFake{session: &service.NextChatManagedSession{UserID: 42, KeyID: 7, APIKey: "chat-key", Purpose: service.NextChatSessionPurposeChat}}
	h := newMobileVideoHandlerWithDependencies(tasks, resolver, execution, &mobileVideoHandlerJobsFake{}, mobileVideoHandlerStorageFake{}, &mobileVideoHandlerBillingFake{})

	response := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", `{"purpose":"video","group_id":9,"model":"video-alpha","prompt":"test","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"r3"}`)
	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	require.Empty(t, tasks.created, "a chat/unlabelled key must never create a video task")
}

func TestMobileVideoHandlerBootstrapReturnsDeclaredServerCapabilities(t *testing.T) {
	bootstrap := &service.MobileVideoBootstrap{ProtocolVersion: 1, CapabilitiesVersion: service.MobileVideoCapabilitiesVersion, Groups: []service.MobileVideoAvailableGroup{{
		ID: 9, Name: "video", VideoAvailable: true, Models: []service.MobileVideoResolvedModel{*mobileVideoHandlerResolvedModel()},
	}}}
	h := newMobileVideoHandlerWithDependencies(&mobileVideoHandlerTasksFake{}, &mobileVideoHandlerResolverFake{bootstrap: bootstrap}, &mobileVideoHandlerExecutionFake{}, &mobileVideoHandlerJobsFake{}, mobileVideoHandlerStorageFake{}, &mobileVideoHandlerBillingFake{})

	response := performMobileVideoRequest(h.Bootstrap, http.MethodGet, "/mobile/video/bootstrap", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var envelope struct {
		Data service.MobileVideoBootstrap `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Equal(t, service.MobileVideoCapabilitiesVersion, envelope.Data.CapabilitiesVersion)
	model := envelope.Data.Groups[0].Models[0]
	require.Equal(t, "video-alpha", model.ID)
	require.Equal(t, "Video Alpha", model.Name)
	require.Equal(t, "video-alpha", model.Model)
	require.Equal(t, []string{"video"}, model.Modalities)
	require.Equal(t, service.MobileVideoCapabilitiesVersion, model.CapabilityVersion)
	var rawEnvelope struct {
		Data struct {
			Groups []struct {
				Models []struct {
					VideoCapabilities json.RawMessage `json:"video_capabilities"`
				} `json:"models"`
			} `json:"groups"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &rawEnvelope))
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rawEnvelope.Data.Groups[0].Models[0].VideoCapabilities, &fields))
	require.Contains(t, fields, "supported_resolutions")
	require.Contains(t, fields, "supported_ratios")
	require.Contains(t, fields, "supported_durations")
	require.NotContains(t, fields, "supported_aspect_ratios")
	require.NotContains(t, fields, "durations_seconds")
}

func TestMobileVideoHandlerAcknowledgeContentReleasesReferenceBeforeBestEffortDelete(t *testing.T) {
	const taskID = "00000000-0000-0000-0000-000000000003"
	jobs := &mobileVideoHandlerJobsFake{jobs: map[string]*service.MobileVideoJob{
		taskID: {
			TaskID: taskID, UserID: 42, State: service.MobileVideoJobStateCompleted,
			ArtifactStorageKey: "private/video.mp4", ArtifactContentType: "video/mp4", ArtifactByteSize: 5,
		},
	}}
	storage := &mobileVideoHandlerAcknowledgementStorageFake{deleteErr: errors.New("object store temporarily unavailable")}
	storage.beforeDelete = func() {
		require.Empty(t, jobs.jobs[taskID].ArtifactStorageKey, "database reference must be cleared before object deletion")
	}
	h := newMobileVideoHandlerWithDependencies(
		&mobileVideoHandlerTasksFake{}, &mobileVideoHandlerResolverFake{}, &mobileVideoHandlerExecutionFake{}, jobs, storage,
		&mobileVideoHandlerBillingFake{},
	)

	first := performMobileVideoRequestWithParam(h.AcknowledgeContent, http.MethodPost, "/mobile/video/jobs/"+taskID+"/content/ack", "", "id", taskID)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Equal(t, []string{taskID}, jobs.releaseCalls)
	require.Equal(t, []string{"private/video.mp4"}, storage.deleteCalls)
	require.Empty(t, jobs.jobs[taskID].ArtifactStorageKey)

	second := performMobileVideoRequestWithParam(h.AcknowledgeContent, http.MethodPost, "/mobile/video/jobs/"+taskID+"/content/ack", "", "id", taskID)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, []string{taskID}, jobs.releaseCalls, "duplicate acknowledgement must not reopen the artifact")
	require.Equal(t, []string{"private/video.mp4"}, storage.deleteCalls, "duplicate acknowledgement must not require a second delete")
}

func TestMobileVideoHandlerAcknowledgeContentDoesNotDeleteWhenReleaseFails(t *testing.T) {
	const taskID = "00000000-0000-0000-0000-000000000004"
	jobs := &mobileVideoHandlerJobsFake{
		releaseErr: errors.New("database unavailable"),
		jobs: map[string]*service.MobileVideoJob{
			taskID: {
				TaskID: taskID, UserID: 42, State: service.MobileVideoJobStateCompleted,
				ArtifactStorageKey: "private/video.mp4", ArtifactContentType: "video/mp4", ArtifactByteSize: 5,
			},
		},
	}
	storage := &mobileVideoHandlerAcknowledgementStorageFake{}
	h := newMobileVideoHandlerWithDependencies(
		&mobileVideoHandlerTasksFake{}, &mobileVideoHandlerResolverFake{}, &mobileVideoHandlerExecutionFake{}, jobs, storage,
		&mobileVideoHandlerBillingFake{},
	)

	response := performMobileVideoRequestWithParam(h.AcknowledgeContent, http.MethodPost, "/mobile/video/jobs/"+taskID+"/content/ack", "", "id", taskID)
	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	require.Equal(t, []string{taskID}, jobs.releaseCalls)
	require.Empty(t, storage.deleteCalls, "storage must not be deleted while its canonical database reference still exists")
	require.Equal(t, "private/video.mp4", jobs.jobs[taskID].ArtifactStorageKey)
}

func TestMobileVideoHandlerCreateFreezesSnapshotReservesOnceAndActivates(t *testing.T) {
	jobs := &mobileVideoHandlerJobsFake{}
	tasks := &mobileVideoHandlerTasksFake{videoJobs: jobs}
	billing := &mobileVideoHandlerBillingFake{}
	execution := &mobileVideoHandlerExecutionFake{
		session:  &service.NextChatManagedSession{UserID: 42, KeyID: 99, APIKey: "video-only", Purpose: service.NextChatSessionPurposeVideo},
		snapshot: mobileVideoHandlerExecutionSnapshot(42, 9, 99, 1.5),
	}
	h := newMobileVideoHandlerWithDependencies(tasks, &mobileVideoHandlerResolverFake{resolved: mobileVideoHandlerResolvedModel()}, execution, jobs, mobileVideoHandlerStorageFake{}, billing)

	created := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", `{"purpose":"video","group_id":9,"model":"video-alpha","prompt":"test","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"funded-create"}`)
	require.Equal(t, http.StatusAccepted, created.Code, created.Body.String())
	require.Len(t, execution.snapshotCalls, 1)
	require.Len(t, tasks.videoInputs, 1)
	require.Equal(t, 0.12, tasks.videoInputs[0].UnitPriceUSD)
	require.Equal(t, 1.5, tasks.videoInputs[0].RateMultiplier)
	require.Equal(t, 1.44, tasks.videoInputs[0].HoldAmount)
	require.Len(t, billing.reserves, 1)
	require.Equal(t, service.BalanceHoldKindMobileVideo, billing.reserves[0].Kind)
	require.Equal(t, 1.44, billing.reserves[0].HoldAmount)
	require.Len(t, jobs.activateCalls, 1)

	snapshotJSON, err := json.Marshal(tasks.videoInputs[0].ExecutionSnapshot)
	require.NoError(t, err)
	require.NotContains(t, string(snapshotJSON), "video-only")

	// A transport retry with the same client request observes the queued durable
	// row and cannot reserve or activate a second time.
	again := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", `{"purpose":"video","group_id":9,"model":"video-alpha","prompt":"test","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"funded-create"}`)
	require.Equal(t, http.StatusAccepted, again.Code, again.Body.String())
	require.Len(t, billing.reserves, 1)
	require.Len(t, jobs.activateCalls, 1)
}

func TestMobileVideoHandlerCreateRejectsInsufficientBalanceWithoutActivation(t *testing.T) {
	jobs := &mobileVideoHandlerJobsFake{}
	tasks := &mobileVideoHandlerTasksFake{videoJobs: jobs}
	billing := &mobileVideoHandlerBillingFake{reserveErr: service.ErrBatchImageInsufficientBalance}
	h := newMobileVideoHandlerWithDependencies(
		tasks, &mobileVideoHandlerResolverFake{resolved: mobileVideoHandlerResolvedModel()},
		&mobileVideoHandlerExecutionFake{session: &service.NextChatManagedSession{UserID: 42, KeyID: 99, APIKey: "video-only", Purpose: service.NextChatSessionPurposeVideo}},
		jobs, mobileVideoHandlerStorageFake{}, billing,
	)

	response := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", `{"purpose":"video","group_id":9,"model":"video-alpha","prompt":"test","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"insufficient"}`)
	require.Equal(t, http.StatusPaymentRequired, response.Code, response.Body.String())
	require.Len(t, billing.reserves, 1)
	require.Empty(t, jobs.activateCalls)
	require.Len(t, jobs.discardCalls, 1)
}

func TestMobileVideoHandlerQueueLimitRefundsBeforeDiscard(t *testing.T) {
	jobs := &mobileVideoHandlerJobsFake{activateErr: service.ErrMobileVideoQueueLimit}
	tasks := &mobileVideoHandlerTasksFake{videoJobs: jobs}
	billing := &mobileVideoHandlerBillingFake{}
	h := newMobileVideoHandlerWithDependencies(
		tasks, &mobileVideoHandlerResolverFake{resolved: mobileVideoHandlerResolvedModel()},
		&mobileVideoHandlerExecutionFake{session: &service.NextChatManagedSession{UserID: 42, KeyID: 99, APIKey: "video-only", Purpose: service.NextChatSessionPurposeVideo}},
		jobs, mobileVideoHandlerStorageFake{}, billing,
	)

	response := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", `{"purpose":"video","group_id":9,"model":"video-alpha","prompt":"test","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"queue-full"}`)
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	require.Len(t, billing.reserves, 1)
	require.Len(t, billing.releases, 1)
	require.Len(t, jobs.discardCalls, 1)
}

func TestMobileVideoHandlerReleaseAcknowledgementFailureNeverReactivatesFundingTask(t *testing.T) {
	jobs := &mobileVideoHandlerJobsFake{
		activateErr:            service.ErrMobileVideoQueueLimit,
		markFundingReleasedErr: errors.New("release state acknowledgement lost"),
	}
	tasks := &mobileVideoHandlerTasksFake{videoJobs: jobs}
	billing := &mobileVideoHandlerBillingFake{}
	h := newMobileVideoHandlerWithDependencies(
		tasks, &mobileVideoHandlerResolverFake{resolved: mobileVideoHandlerResolvedModel()},
		&mobileVideoHandlerExecutionFake{session: &service.NextChatManagedSession{UserID: 42, KeyID: 99, APIKey: "video-only", Purpose: service.NextChatSessionPurposeVideo}},
		jobs, mobileVideoHandlerStorageFake{}, billing,
	)
	body := `{"purpose":"video","group_id":9,"model":"video-alpha","prompt":"test","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"release-ack-lost"}`

	first := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", body)
	require.Equal(t, http.StatusServiceUnavailable, first.Code, first.Body.String())
	require.Len(t, billing.reserves, 1)
	require.Len(t, billing.releases, 1)
	require.Len(t, jobs.activateCalls, 1)
	require.Len(t, jobs.fundingReleaseCalls, 1)
	job := jobs.jobs["00000000-0000-0000-0000-000000000001"]
	require.NotNil(t, job)
	require.Equal(t, service.MobileVideoBillingStateReleasing, job.BillingState)

	// The original release may have committed despite losing the state-write
	// reply. Retrying the request may retry the idempotent release, but must
	// only complete compensation; it must never activate an unfunded task.
	jobs.markFundingReleasedErr = nil
	jobs.activateErr = nil
	retry := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", body)
	require.Equal(t, http.StatusConflict, retry.Code, retry.Body.String())
	require.Len(t, billing.reserves, 1)
	require.Len(t, billing.releases, 2)
	require.Len(t, jobs.activateCalls, 1)
	require.Len(t, jobs.discardCalls, 1)
	require.NotContains(t, jobs.jobs, "00000000-0000-0000-0000-000000000001")
}

func TestMobileVideoHandlerCancelMarksClientCancellationWithoutTerminatingPrivateJob(t *testing.T) {
	const taskID = "00000000-0000-0000-0000-000000000005"
	tasks := &mobileVideoHandlerTasksFake{tasks: map[string]*service.MobileTask{}}
	task, err := service.NewMobileTask(taskID, service.MobileTaskKindVideo, service.MobileVideoOperationGenerate, "cancel-handler", time.Now().UTC())
	require.NoError(t, err)
	tasks.tasks[taskID] = &task
	jobs := &mobileVideoHandlerJobsFake{jobs: map[string]*service.MobileVideoJob{
		taskID: {TaskID: taskID, UserID: 42, State: service.MobileVideoJobStatePolling, BillingState: service.MobileVideoBillingStateReserved},
	}}
	h := newMobileVideoHandlerWithDependencies(tasks, &mobileVideoHandlerResolverFake{}, &mobileVideoHandlerExecutionFake{}, jobs, mobileVideoHandlerStorageFake{}, &mobileVideoHandlerBillingFake{})

	response := performMobileVideoRequestWithParam(h.Cancel, http.MethodPost, "/mobile/video/jobs/"+taskID+"/cancel", "", "id", taskID)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, []string{taskID}, jobs.clientCancellationCalls)
	require.Empty(t, jobs.markCancelledCalls)
	require.Equal(t, service.MobileVideoJobStatePolling, jobs.jobs[taskID].State)
}

func TestMobileVideoHandlerCreateRejectsOversizedRequestBody(t *testing.T) {
	h := newMobileVideoHandlerWithDependencies(
		&mobileVideoHandlerTasksFake{}, &mobileVideoHandlerResolverFake{resolved: mobileVideoHandlerResolvedModel()},
		&mobileVideoHandlerExecutionFake{session: &service.NextChatManagedSession{UserID: 42, KeyID: 99, APIKey: "video-only", Purpose: service.NextChatSessionPurposeVideo}},
		&mobileVideoHandlerJobsFake{}, mobileVideoHandlerStorageFake{}, &mobileVideoHandlerBillingFake{},
	)
	router := gin.New()
	router.POST("/mobile/video/jobs", middleware.RequestBodyLimit(MobileVideoJobRequestBodyLimit), func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		h.Create(c)
	})
	body := `{"purpose":"video","group_id":9,"model":"video-alpha","prompt":"` +
		strings.Repeat("x", int(MobileVideoJobRequestBodyLimit)) +
		`","resolution":"720p","ratio":"16:9","duration_seconds":8,"client_request_id":"too-large"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/mobile/video/jobs", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
	var envelope struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "MOBILE_VIDEO_REQUEST_TOO_LARGE", envelope.Reason)
}

func performMobileVideoRequest(method gin.HandlerFunc, httpMethod, target, body string) *httptest.ResponseRecorder {
	return performMobileVideoRequestWithParam(method, httpMethod, target, body, "", "")
}

func performMobileVideoRequestWithParam(method gin.HandlerFunc, httpMethod, target, body, param, value string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(httpMethod, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	if param != "" {
		context.Params = []gin.Param{{Key: param, Value: value}}
	}
	context.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	method(context)
	return recorder
}
