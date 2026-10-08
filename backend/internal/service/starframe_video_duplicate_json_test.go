//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStarframeRejectsDuplicateJSONBillingAndMaterialFields(t *testing.T) {
	for _, body := range []string{
		`{"model":"ch-priced","model":"ch-unpriced","prompt":"p","mode":"references","client_task_id":"x","duration":5,"resolution":"720p"}`,
		`{"model":"ch-priced","prompt":"p","mode":"references","client_task_id":"x","duration":5,"duration":15,"resolution":"720p"}`,
		`{"model":"ch-priced","prompt":"p","mode":"references","client_task_id":"x","duration":5,"resolution":"720p","resolution":"1080p"}`,
		`{"model":"ch-priced","prompt":"p","mode":"references","client_task_id":"x","client_task_id":"y","duration":5,"resolution":"720p"}`,
		`{"model":"ch-priced","prompt":"p","mode":"references","client_task_id":"x","duration":5,"resolution":"720p","references":{"image":"https://example.com/a.png","image":"data:image/png;base64,xxx"}}`,
	} {
		_, err := ParseStarframeVideoRequest([]byte(body))
		require.Error(t, err, body)
	}
}
