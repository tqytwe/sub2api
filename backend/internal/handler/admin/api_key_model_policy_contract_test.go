package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminAPIKeyPolicyContractRejectsInvalidPolicy(t *testing.T) {
	for _, payload := range []string{
		`{"reasoning_effort":"invalid"}`,
		`{"model_mapping":{"public":"bad target"}}`,
		`{"context_token_limit":-1}`,
	} {
		t.Run(payload, func(t *testing.T) {
			router := setupAPIKeyHandler(newStubAdminService())
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/api-keys/10", bytes.NewBufferString(payload))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusBadRequest, recorder.Code, "administrator policy must not be silently ignored")
		})
	}
}
