package requestledger

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCorrelationIdentifierBoundsAndProtocol(t *testing.T) {
	for _, value := range []string{"", strings.Repeat("a", 129), "request\nid", " id", "é", "sk-secret", "sess-secret", "Bearer-secret", "eyJhbGci-secret"} {
		require.Empty(t, UpstreamIdentifier(value), value)
	}
	require.Equal(t, strings.Repeat("a", 128), UpstreamIdentifier(strings.Repeat("a", 128)))
	for _, body := range []string{`{"type":"error","id":"event_1"}`, `{"event_id":"client_supplied"}`, `{"input":{"id":"private_prompt"}}`, `{"id":"ambiguous"}`} {
		require.Empty(t, ResponseIdentifier([]byte(body)))
	}
	for _, body := range []string{`{"type":"response.failed","response":{"id":"resp_1"}}`, `{"object":"response","id":"resp_1"}`, `{"object":"chat.completion.chunk","id":"resp_1"}`, `{"type":"message_start","message":{"id":"resp_1"}}`, `{"responseId":"resp_1"}`} {
		require.Equal(t, "resp_1", ResponseIdentifier([]byte(body)))
	}
	var forged Receipt
	//nolint:staticcheck // Intentionally prove untrusted JSON cannot construct the private capability.
	require.NoError(t, json.Unmarshal([]byte(`{"request_id":"forged","attempt_id":"forged","attempt_no":1}`), &forged))
	require.Nil(t, forged.Correlation(), "JSON metadata cannot construct a trusted receipt")
}

func TestStreamCorrelationIndependentOfReadBoundaries(t *testing.T) {
	data := []byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_first\"}}\n\ndata: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_second\"}}\n\n")
	for _, size := range []int{1, 7, len(data)} {
		var ids []string
		evidence := streamEvidence{observeID: func(id string) { ids = append(ids, id) }}
		for pos := 0; pos < len(data); pos += size {
			evidence.observe(data[pos:min(pos+size, len(data))])
		}
		require.Equal(t, []string{"resp_first", "resp_second"}, ids)
		require.Equal(t, "failed", evidence.terminal)
	}
}
