package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestLedgerWSEndTypedFailoverAndSentinelPriority(t *testing.T) {
	cause := &UpstreamFailoverError{StatusCode: 429, ResponseBody: []byte("body-secret-sentinel")}
	require.Equal(t, "failover", ledgerWSEndReason(cause))
	require.Equal(t, "ingress_lease_lost", ledgerWSEndReason(errors.Join(context.Canceled, ErrOpenAIWSIngressLeaseLost)))
	require.Equal(t, "session_preempted", ledgerWSEndReason(errors.Join(context.Canceled, errOpenAIWSSessionPreempted)))
	require.Equal(t, "unknown", ledgerWSEndReason(errors.New("context canceled read_upstream body-secret-sentinel")), "never classify arbitrary error text")
}

func TestRequestLedgerWSEndPassthroughTimeoutCauses(t *testing.T) {
	for _, cause := range []error{&openAIWSPassthroughFirstOutputTimeoutError{}, &openAIWSPassthroughActiveTurnTimeoutError{}} {
		require.Equal(t, "timeout", ledgerWSEndReason(cause))
		require.Equal(t, "timeout", ledgerWSEndReason(fmt.Errorf("secret-sentinel: %w", cause)))
	}
}
