//go:build unit

package handler

import (
	"bytes"
	"compress/gzip"
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

func TestBindBatchImageSubmitRequestAcceptsUTF8BOMJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"model":"gemini-image-test","items":[{"custom_id":"one","prompt":"cat"}]}`)...)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/batches", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")

	var request service.BatchImageSubmitRequest
	err := bindBatchImageSubmitRequest(context, &request)

	require.NoError(t, err)
	require.Equal(t, "gemini-image-test", request.Model)
	require.Len(t, request.Items, 1)
	require.Equal(t, "cat", request.Items[0].Prompt)
}

func TestBatchImageSubmitBindErrorPreservesRequestTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/batches", bytes.NewBufferString(`{"items":[]}`))
	context.Request.Body = http.MaxBytesReader(recorder, context.Request.Body, 4)
	context.Request.Header.Set("Content-Type", "application/json")

	var request service.BatchImageSubmitRequest
	err := bindBatchImageSubmitRequest(context, &request)
	writeBatchImageSubmitBindError(context, err)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Contains(t, recorder.Body.String(), "BATCH_IMAGE_REQUEST_TOO_LARGE")
}

func TestBatchImageSubmitBindErrorDistinguishesEncodingAndJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("broken gzip", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/batches", bytes.NewBufferString("not-gzip"))
		context.Request.Header.Set("Content-Type", "application/json")
		context.Request.Header.Set("Content-Encoding", "gzip")

		var request service.BatchImageSubmitRequest
		err := bindBatchImageSubmitRequest(context, &request)
		writeBatchImageSubmitBindError(context, err)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Contains(t, recorder.Body.String(), "BATCH_IMAGE_REQUEST_DECODE_FAILED")
	})

	t.Run("malformed json", func(t *testing.T) {
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		_, err := writer.Write([]byte(`{"items":`))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/batches", &compressed)
		context.Request.Header.Set("Content-Type", "application/json")
		context.Request.Header.Set("Content-Encoding", "gzip")

		var request service.BatchImageSubmitRequest
		err = bindBatchImageSubmitRequest(context, &request)
		writeBatchImageSubmitBindError(context, err)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Contains(t, recorder.Body.String(), "BATCH_IMAGE_INVALID_JSON")
	})
}
