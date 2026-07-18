//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWriteBatchImageAcceptedSetsPollingContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	batch := &service.BatchImagePublicBatch{ID: "imgbatch_contract"}

	writeBatchImageAccepted(context, batch)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, "/v1/images/batches/imgbatch_contract", recorder.Header().Get("Location"))
	require.Equal(t, "5", recorder.Header().Get("Retry-After"))
}
