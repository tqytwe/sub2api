package service

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

const (
	imageStudioHoldRequestPrefix    = "image_studio_hold:"
	imageStudioCaptureRequestPrefix = "image_studio_capture:"
	imageStudioReleaseRequestPrefix = "image_studio_release:"
)

func WithImageStudioManagedBilling(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxkey.ImageStudioManagedBilling, true)
}

func IsImageStudioManagedBilling(ctx context.Context) bool {
	managed, _ := ctx.Value(ctxkey.ImageStudioManagedBilling).(bool)
	return managed
}

type ImageStudioBillingCapture struct {
	once sync.Once
	ch   chan float64
}

func NewImageStudioBillingCapture() *ImageStudioBillingCapture {
	return &ImageStudioBillingCapture{ch: make(chan float64, 1)}
}

func WithImageStudioBillingCapture(ctx context.Context, capture *ImageStudioBillingCapture) context.Context {
	ctx = WithImageStudioManagedBilling(ctx)
	return context.WithValue(ctx, ctxkey.ImageStudioBillingCapture, capture)
}

func ImageStudioBillingCaptureFromContext(ctx context.Context) *ImageStudioBillingCapture {
	capture, _ := ctx.Value(ctxkey.ImageStudioBillingCapture).(*ImageStudioBillingCapture)
	return capture
}

func WithImageStudioBillingActualCostCap(ctx context.Context, cap float64) context.Context {
	if cap < 0 {
		cap = 0
	}
	return context.WithValue(ctx, ctxkey.ImageStudioBillingActualCostCap, cap)
}

func applyImageStudioBillingActualCostCap(ctx context.Context, cost *CostBreakdown) {
	if !IsImageStudioManagedBilling(ctx) || cost == nil {
		return
	}
	cap, ok := ctx.Value(ctxkey.ImageStudioBillingActualCostCap).(float64)
	if !ok {
		return
	}
	if cap < 0 {
		cap = 0
	}
	if cost.ActualCost > cap {
		cost.ActualCost = cap
	}
}

func ImageStudioPerItemBillingCap(job *ImageStudioJob) float64 {
	if job == nil {
		return 0
	}
	held := job.EstimatedCost
	if job.HoldAmount != nil {
		held = *job.HoldAmount
	}
	if held < 0 {
		held = 0
	}
	count := job.Count
	if count <= 0 {
		count = 1
	}
	return held / float64(count)
}

func RecordImageStudioManagedBillingCost(ctx context.Context, actualCost float64) {
	if !IsImageStudioManagedBilling(ctx) {
		return
	}
	capture := ImageStudioBillingCaptureFromContext(ctx)
	if capture == nil {
		return
	}
	if actualCost < 0 {
		actualCost = 0
	}
	capture.once.Do(func() {
		capture.ch <- actualCost
	})
}

func recordImageStudioManagedUsageForReconciliation(ctx context.Context, repo UsageLogRepository, usageLog *UsageLog, component string, actualCost float64) {
	if !IsImageStudioManagedBilling(ctx) {
		return
	}
	writeUsageLogBestEffort(ctx, repo, usageLog, component)
	RecordImageStudioManagedBillingCost(ctx, actualCost)
}

func (c *ImageStudioBillingCapture) Wait(ctx context.Context) (float64, bool) {
	if c == nil {
		return 0, false
	}
	select {
	case cost := <-c.ch:
		return cost, true
	case <-ctx.Done():
		return 0, false
	}
}

func ImageStudioHoldRequestID(jobID string) string {
	return imageStudioHoldRequestPrefix + strings.TrimSpace(jobID)
}

func ImageStudioCaptureRequestID(jobID string) string {
	return imageStudioCaptureRequestPrefix + strings.TrimSpace(jobID)
}

func ImageStudioReleaseRequestID(jobID string) string {
	return imageStudioReleaseRequestPrefix + strings.TrimSpace(jobID)
}

func buildImageStudioHoldCommand(job *ImageStudioJob, requestID string, actualAmount float64) (*BatchImageBalanceHoldCommand, error) {
	if job == nil || job.APIKeyID == nil || *job.APIKeyID <= 0 {
		return nil, ErrImageStudioBillingFailed
	}
	holdAmount := job.EstimatedCost
	if job.HoldAmount != nil {
		holdAmount = *job.HoldAmount
	}
	if holdAmount < 0 {
		holdAmount = 0
	}
	if actualAmount < 0 {
		actualAmount = 0
	}
	holdID := strings.TrimSpace(job.HoldID)
	if holdID == "" {
		holdID = ImageStudioHoldRequestID(job.ID)
	}
	return &BatchImageBalanceHoldCommand{
		RequestID:           requestID,
		HoldRequestID:       holdID,
		CaptureRequestID:    ImageStudioCaptureRequestID(job.ID),
		ReleaseRequestID:    ImageStudioReleaseRequestID(job.ID),
		APIKeyID:            *job.APIKeyID,
		UserID:              job.UserID,
		BatchID:             job.ID,
		HoldAmount:          holdAmount,
		ActualAmount:        actualAmount,
		AllowBalanceOverage: false,
		RequestPayloadHash:  job.PromptHash,
	}, nil
}

func reserveImageStudioBalance(ctx context.Context, repo UsageBillingRepository, job *ImageStudioJob) error {
	if repo == nil {
		return ErrImageStudioBillingFailed.WithCause(errors.New("usage billing repository is not configured"))
	}
	cmd, err := buildImageStudioHoldCommand(job, ImageStudioHoldRequestID(job.ID), 0)
	if err != nil {
		return err
	}
	if _, err := repo.ReserveBatchImageBalance(ctx, cmd); err != nil {
		if errors.Is(err, ErrBatchImageInsufficientBalance) {
			return ErrImageStudioInsufficientBalance
		}
		return ErrImageStudioBillingFailed.WithCause(err)
	}
	return nil
}

func settleImageStudioBalance(ctx context.Context, repo UsageBillingRepository, job *ImageStudioJob, actualAmount float64) error {
	if repo == nil || job == nil {
		return ErrImageStudioBillingFailed
	}
	requestID := ImageStudioCaptureRequestID(job.ID)
	if actualAmount <= 0 {
		requestID = ImageStudioReleaseRequestID(job.ID)
	}
	cmd, err := buildImageStudioHoldCommand(job, requestID, actualAmount)
	if err != nil {
		return err
	}
	if actualAmount <= 0 {
		_, err = repo.ReleaseBatchImageBalance(ctx, cmd)
	} else {
		_, err = repo.CaptureBatchImageBalance(ctx, cmd)
	}
	if err != nil {
		return ErrImageStudioBillingFailed.WithCause(err)
	}
	return nil
}

func (s *ImageStudioService) ReconcileBilling(ctx context.Context, limit int) (int, error) {
	if s == nil || s.billingRepo == nil {
		return 0, nil
	}
	reconciler, ok := s.billingRepo.(ImageStudioBillingReconciler)
	if !ok {
		return 0, nil
	}
	return reconciler.ReconcileImageStudioBilling(ctx, limit)
}
