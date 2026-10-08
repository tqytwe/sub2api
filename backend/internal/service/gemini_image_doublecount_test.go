package service

import (
	"testing"

	"github.com/tidwall/gjson"
)

// TestConvertGeminiNativeImageResponseToOpenAIImages_NoDuplication tests that
// convertGeminiNativeImageResponseToOpenAIImages does not double-count images
// when Gemini response contains both inlineData (camelCase) and inline_data
// (snake_case) fields pointing to the same image.
func TestConvertGeminiNativeImageResponseToOpenAIImages_NoDuplication(t *testing.T) {
	tests := []struct {
		name          string
		geminiJSON    string
		expectedCount int
		expectError   bool
	}{
		{
			name: "single image with only inlineData (camelCase)",
			geminiJSON: `{
				"candidates": [{
					"content": {
						"parts": [{
							"inlineData": {
								"mimeType": "image/png",
								"data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
							}
						}]
					}
				}]
			}`,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "single image with only inline_data (snake_case)",
			geminiJSON: `{
				"candidates": [{
					"content": {
						"parts": [{
							"inline_data": {
								"mimeType": "image/png",
								"data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
							}
						}]
					}
				}]
			}`,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "CRITICAL: single image with BOTH inlineData and inline_data (same content)",
			geminiJSON: `{
				"candidates": [{
					"content": {
						"parts": [{
							"inlineData": {
								"mimeType": "image/png",
								"data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
							},
							"inline_data": {
								"mimeType": "image/png",
								"data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
							}
						}]
					}
				}]
			}`,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "two distinct images in separate parts",
			geminiJSON: `{
				"candidates": [{
					"content": {
						"parts": [
							{
								"inlineData": {
									"mimeType": "image/png",
									"data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
								}
							},
							{
								"inlineData": {
									"mimeType": "image/jpeg",
									"data": "/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAv/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwCwAA0AAf/Z"
								}
							}
						]
					}
				}]
			}`,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "mixed aliases in separate parts preserve both images",
			geminiJSON: `{
				"candidates": [{
					"content": {
						"parts": [
							{
								"inlineData": {
									"mimeType": "image/png",
									"data": "Y2FtZWw="
								}
							},
							{
								"inline_data": {
									"mime_type": "image/webp",
									"data": "c25ha2U="
								}
							}
						]
					}
				}]
			}`,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "empty response",
			geminiJSON: `{
				"candidates": []
			}`,
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respBody, count, err := convertGeminiNativeImageResponseToOpenAIImages([]byte(tt.geminiJSON))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if count != tt.expectedCount {
				t.Errorf("image count = %d, want %d", count, tt.expectedCount)
			}

			if respBody == nil {
				t.Error("respBody is nil")
			}
			if got := int(gjson.GetBytes(respBody, "data.#").Int()); got != tt.expectedCount {
				t.Errorf("response image count = %d, want %d", got, tt.expectedCount)
			}
		})
	}
}
