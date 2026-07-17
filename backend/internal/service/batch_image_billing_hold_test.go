//go:build unit

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchImageBalanceHoldFingerprint_IgnoresHoldRequestIDForBackwardCompatibility(t *testing.T) {
	cmd := &BatchImageBalanceHoldCommand{
		UserID:             42,
		APIKeyID:           84,
		BatchID:            " imgbatch_legacy ",
		HoldRequestID:      " batch_image_hold:imgbatch_legacy ",
		HoldAmount:         1.25,
		ActualAmount:       0.75,
		RequestPayloadHash: " payload-hash ",
	}

	cmd.Normalize()

	raw := fmt.Sprintf(
		"%d|%d|%s|%0.10f|%0.10f|%s",
		cmd.UserID,
		cmd.APIKeyID,
		strings.TrimSpace(cmd.BatchID),
		cmd.HoldAmount,
		cmd.ActualAmount,
		strings.TrimSpace(cmd.RequestPayloadHash),
	)
	sum := sha256.Sum256([]byte(raw))
	require.Equal(t, hex.EncodeToString(sum[:]), cmd.RequestFingerprint)

	withDifferentHoldOwner := *cmd
	withDifferentHoldOwner.HoldRequestID = "batch_image_hold:another_batch"
	withDifferentHoldOwner.RequestFingerprint = ""
	withDifferentHoldOwner.Normalize()
	require.Equal(t, cmd.RequestFingerprint, withDifferentHoldOwner.RequestFingerprint)
}

func TestBuildBatchImageHoldCommand_PreservesIndependentHoldOwnershipID(t *testing.T) {
	apiKeyID := int64(84)
	holdAmount := 1.25
	job := &BatchImageJob{
		BatchID:    "imgbatch_ownership",
		UserID:     42,
		APIKeyID:   &apiKeyID,
		HoldAmount: &holdAmount,
	}

	cmd, err := buildBatchImageHoldCommand(
		job,
		BatchImageCaptureRequestID(job.BatchID),
		0.75,
		"payload-hash",
	)

	require.NoError(t, err)
	require.Equal(t, BatchImageHoldRequestID(job.BatchID), cmd.HoldRequestID)
}
