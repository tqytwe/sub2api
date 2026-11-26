package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsOpenAIInvalidEncryptedContentError(t *testing.T) {
	for _, tc := range []struct {
		code, message string
		want          bool
	}{
		{"invalid_encrypted_content", "provider rejected the encrypted content", true},
		{" INVALID_ENCRYPTED_CONTENT ", "provider rejected the encrypted content", true},
		{"thinking_signature_invalid", "The encrypted content could not be verified because it could not be decrypted or parsed", true},
		{"thinking_signature_invalid", "unrelated signature error", false},
		{"invalid_request_error", "The encrypted content could not be verified", false},
		{"", "The encrypted content could not be verified and could not be decrypted or parsed", false},
	} {
		t.Run(tc.code+"/"+tc.message, func(t *testing.T) {
			require.Equal(t, tc.want, isOpenAIInvalidEncryptedContentError(tc.code, tc.message))
		})
	}
}

func TestOpenAIRejectedEncryptedDigestsRequiresSafeEvidence(t *testing.T) {
	const rejection = `{"error":{"code":"invalid_encrypted_content","message":"The encrypted content could not be verified."}}`
	oneReasoning := []byte(`{"input":[{"type":"reasoning","encrypted_content":"fixture-ciphertext-one"}]}`)
	require.Equal(t, []string{openAIEncryptedContentDigest("fixture-ciphertext-one")}, openAIRejectedEncryptedDigests(oneReasoning, []byte(rejection)))

	withAgentHistory := []byte(`{"input":[{"type":"agent_message","encrypted_content":"fixture-agent-ciphertext","content":"visible"},{"type":"reasoning","encrypted_content":"fixture-ciphertext-one"}]}`)
	require.Empty(t, openAIRejectedEncryptedDigests(withAgentHistory, []byte(rejection)), "agent history prevents an ambiguous reasoning-only retry")

	multipleCandidates := []byte(`{"input":[{"type":"reasoning","encrypted_content":"fixture-ciphertext-one"},{"type":"reasoning","encrypted_content":"fixture-ciphertext-two"}]}`)
	require.Empty(t, openAIRejectedEncryptedDigests(multipleCandidates, []byte(rejection)), "generic errors cannot select among multiple ciphertexts")
	require.Empty(t, openAIRejectedEncryptedDigests(oneReasoning, []byte(`{"error":{"code":"invalid_request_error","message":"unsupported field"}}`)))
	require.Empty(t, openAIRejectedEncryptedDigests([]byte(`{"input":[{"type":"reasoning","encrypted_content":"x"}],"input":[]}`), []byte(rejection)))
}

func TestOpenAIRejectedEncryptedDigestsRetainsPreciseCompactionCompatibility(t *testing.T) {
	rejection := []byte(`{"error":{"code":"invalid_encrypted_content","message":"bad encrypted content"}}`)
	one := []byte(`{"input":[{"type":"compaction","encrypted_content":"fixture-compaction"},{"type":"message","role":"user","content":"continue"}]}`)
	require.Equal(t, []string{openAIEncryptedContentDigest("fixture-compaction")}, openAIRejectedEncryptedDigests(one, rejection))
	ambiguous := []byte(`{"input":[{"type":"compaction","encrypted_content":"fixture-compaction"},{"type":"reasoning","encrypted_content":"fixture-reasoning"}]}`)
	require.Empty(t, openAIRejectedEncryptedDigests(ambiguous, rejection), "generic rejections must not strip multiple history items")
}

func TestOpenAIRejectedEncryptedDigestsMatchesUniqueLocator(t *testing.T) {
	one := "ABCD-one-1234"
	two := "WXYZ-two-5678"
	body := []byte(`{"input":[{"type":"reasoning","encrypted_content":"` + one + `"},{"type":"reasoning","encrypted_content":"` + two + `"}]}`)
	rejection := []byte(`{"error":{"code":"invalid_encrypted_content","message":"The encrypted content WXYZ...5678 could not be verified. Reason: Encrypted content could not be decrypted or parsed."}}`)
	require.Equal(t, []string{openAIEncryptedContentDigest(two)}, openAIRejectedEncryptedDigests(body, rejection))
	wrong := []byte(`{"error":{"code":"invalid_encrypted_content","message":"The encrypted content DEAD...BEEF could not be verified. Reason: Encrypted content could not be decrypted or parsed."}}`)
	require.Empty(t, openAIRejectedEncryptedDigests(body, wrong), "a locator mismatch does not fall back to an unrelated candidate")
}
