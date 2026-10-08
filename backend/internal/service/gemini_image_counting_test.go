package service

import (
	"encoding/json"
	"testing"
)

func TestConvertGeminiNativeImageResponseToOpenAIImages_NoDuplicates(t *testing.T) {
	tests := []struct {
		name          string
		geminiResp    string
		wantImageCount int
	}{
		{
			name: "single image with inlineData should count as 1",
			geminiResp: `{
				"candidates": [
					{
						"content": {
							"parts": [
								{
									"inlineData": {
										"mimeType": "image/png",
										"data": "base64data1"
									}
								}
							]
						}
					}
				]
			}`,
			wantImageCount: 1,
		},
		{
			name: "single image with inline_data should count as 1",
			geminiResp: `{
				"candidates": [
					{
						"content": {
							"parts": [
								{
									"inline_data": {
										"mimeType": "image/png",
										"data": "base64data1"
									}
								}
							]
						}
					}
				]
			}`,
			wantImageCount: 1,
		},
		{
			name: "two distinct images should count as 2",
			geminiResp: `{
				"candidates": [
					{
						"content": {
							"parts": [
								{
									"inlineData": {
										"mimeType": "image/png",
										"data": "base64data1"
									}
								},
								{
									"inlineData": {
										"mimeType": "image/png",
										"data": "base64data2"
									}
								}
							]
						}
					}
				]
			}`,
			wantImageCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respBody, gotCount, err := convertGeminiNativeImageResponseToOpenAIImages([]byte(tt.geminiResp))
			if err != nil {
				t.Fatalf("convertGeminiNativeImageResponseToOpenAIImages() error = %v", err)
			}

			if gotCount != tt.wantImageCount {
				t.Errorf("convertGeminiNativeImageResponseToOpenAIImages() count = %v, want %v", gotCount, tt.wantImageCount)
			}

			var openAIResp map[string]any
			if err := json.Unmarshal(respBody, &openAIResp); err != nil {
				t.Fatalf("failed to parse OpenAI response: %v", err)
			}

			data, ok := openAIResp["data"].([]any)
			if !ok {
				t.Fatal("response data field is not an array")
			}

			if len(data) != tt.wantImageCount {
				t.Errorf("response data length = %v, want %v", len(data), tt.wantImageCount)
			}
		})
	}
}
