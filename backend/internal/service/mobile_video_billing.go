package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

const (
	mobileVideoHoldRequestPrefix    = "mobile_video_hold:"
	mobileVideoCaptureRequestPrefix = "mobile_video_capture:"
	mobileVideoReleaseRequestPrefix = "mobile_video_release:"

	// MobileVideoActiveJobsPerUser caps accepted but unfinished video jobs. The
	// cap is enforced by the durable activation transaction, not by an
	// in-memory counter, so multiple server replicas cannot oversubscribe a
	// user's frozen balance.
	MobileVideoActiveJobsPerUser = 3
)

func WithMobileVideoManagedExecution(ctx context.Context) context.Context {
	ctx = WithImageStudioManagedBilling(ctx)
	return context.WithValue(ctx, ctxkey.MobileVideoManagedExecution, true)
}

func IsMobileVideoManagedExecution(ctx context.Context) bool {
	managed, _ := ctx.Value(ctxkey.MobileVideoManagedExecution).(bool)
	return managed
}

func MobileVideoHoldRequestID(taskID string) string {
	return mobileVideoHoldRequestPrefix + strings.TrimSpace(taskID)
}

func MobileVideoCaptureRequestID(taskID string) string {
	return mobileVideoCaptureRequestPrefix + strings.TrimSpace(taskID)
}

func MobileVideoReleaseRequestID(taskID string) string {
	return mobileVideoReleaseRequestPrefix + strings.TrimSpace(taskID)
}

// BuildMobileVideoBalanceHoldCommand produces stable ids for the three phases
// of one accepted video task. It uses the existing transactionally idempotent
// hold ledger, with a distinct audit kind so image statements remain unchanged.
func BuildMobileVideoBalanceHoldCommand(job *MobileVideoJob, requestID string, actualAmount float64) (*BatchImageBalanceHoldCommand, error) {
	if job == nil || job.UserID <= 0 || job.ExecutionAPIKeyID <= 0 || strings.TrimSpace(job.TaskID) == "" ||
		math.IsNaN(job.HoldAmount) || math.IsInf(job.HoldAmount, 0) || job.HoldAmount < 0 {
		return nil, ErrMobileVideoFundingInvalid
	}
	if math.IsNaN(actualAmount) || math.IsInf(actualAmount, 0) || actualAmount < 0 {
		return nil, ErrMobileVideoFundingInvalid
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, ErrMobileVideoFundingInvalid
	}
	return &BatchImageBalanceHoldCommand{
		Kind:                BalanceHoldKindMobileVideo,
		RequestID:           requestID,
		HoldRequestID:       MobileVideoHoldRequestID(job.TaskID),
		CaptureRequestID:    MobileVideoCaptureRequestID(job.TaskID),
		ReleaseRequestID:    MobileVideoReleaseRequestID(job.TaskID),
		APIKeyID:            job.ExecutionAPIKeyID,
		UserID:              job.UserID,
		BatchID:             job.TaskID,
		HoldAmount:          QuantizeUsageBillingAmount(job.HoldAmount),
		ActualAmount:        QuantizeUsageBillingAmount(actualAmount),
		AllowBalanceOverage: false,
		RequestPayloadHash:  mobileVideoPromptHash(job.Prompt),
	}, nil
}

func ReserveMobileVideoBalance(ctx context.Context, repo UsageBillingRepository, job *MobileVideoJob) error {
	if repo == nil {
		return ErrMobileVideoFundingInvalid
	}
	cmd, err := BuildMobileVideoBalanceHoldCommand(job, MobileVideoHoldRequestID(job.TaskID), 0)
	if err != nil {
		return err
	}
	_, err = repo.ReserveBatchImageBalance(ctx, cmd)
	return err
}

func CaptureMobileVideoBalance(ctx context.Context, repo UsageBillingRepository, job *MobileVideoJob) error {
	if repo == nil {
		return ErrMobileVideoFundingInvalid
	}
	cmd, err := BuildMobileVideoBalanceHoldCommand(job, MobileVideoCaptureRequestID(job.TaskID), job.HoldAmount)
	if err != nil {
		return err
	}
	_, err = repo.CaptureBatchImageBalance(ctx, cmd)
	return err
}

func ReleaseMobileVideoBalance(ctx context.Context, repo UsageBillingRepository, job *MobileVideoJob) error {
	if repo == nil {
		return ErrMobileVideoFundingInvalid
	}
	cmd, err := BuildMobileVideoBalanceHoldCommand(job, MobileVideoReleaseRequestID(job.TaskID), 0)
	if err != nil {
		return err
	}
	_, err = repo.ReleaseBatchImageBalance(ctx, cmd)
	return err
}

// ValidateMobileVideoHoldAmount keeps the arithmetic used in the handler in
// one place and prevents a malformed catalog price from creating a negative or
// non-finite hold.
func ValidateMobileVideoHoldAmount(unitPrice float64, duration int, multiplier float64) (float64, error) {
	if math.IsNaN(unitPrice) || math.IsInf(unitPrice, 0) ||
		math.IsNaN(multiplier) || math.IsInf(multiplier, 0) ||
		unitPrice < 0 || duration <= 0 || multiplier < 0 {
		return 0, ErrMobileVideoFundingInvalid
	}
	hold := QuantizeUsageBillingAmount(unitPrice * float64(duration) * multiplier)
	if math.IsNaN(hold) || math.IsInf(hold, 0) || hold < 0 {
		return 0, ErrMobileVideoFundingInvalid
	}
	return hold, nil
}

// MobileVideoFundingError keeps handler error mapping narrow while preserving
// the underlying idempotency/ownership error for logs and tests.
func MobileVideoFundingError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("mobile video %s funding: %w", strings.TrimSpace(operation), err)
}

func IsMobileVideoInsufficientBalance(err error) bool {
	return errors.Is(err, ErrBatchImageInsufficientBalance)
}
