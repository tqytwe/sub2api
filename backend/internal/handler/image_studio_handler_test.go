package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestImageStudioSingleImageRequestBodyForcesOneImage(t *testing.T) {
	body := `{"model":"gpt-image-2","prompt":"draw a cat","n":4,"size":"1024x1024"}`

	require.Equal(t, 4, imageStudioGenerationCount(body))

	got, err := imageStudioSingleImageRequestBody(body)
	require.NoError(t, err)
	require.Equal(t, int64(1), gjson.Get(got, "n").Int())
	require.Equal(t, "gpt-image-2", gjson.Get(got, "model").String())
	require.Equal(t, "draw a cat", gjson.Get(got, "prompt").String())
	require.Equal(t, "1024x1024", gjson.Get(got, "size").String())
}

func TestImageStudioGenerationCountDefaultsToOne(t *testing.T) {
	require.Equal(t, 1, imageStudioGenerationCount(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
	require.Equal(t, 1, imageStudioGenerationCount(`{"model":"gpt-image-2","prompt":"draw a cat","n":0}`))
}
