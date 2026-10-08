package service

import (
	"testing"
)

func TestGeminiNanaBananaImageModelValidation(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantValid bool
	}{
		{
			name:      "gemini-nano-banana-2.1-2k should be recognized",
			model:     "gemini-nano-banana-2.1-2k",
			wantValid: true,
		},
		{
			name:      "gemini-nano-banana-2.1 should be recognized",
			model:     "gemini-nano-banana-2.1",
			wantValid: true,
		},
		{
			name:      "GEMINI-NANO-BANANA-2.1-2K uppercase should work",
			model:     "GEMINI-NANO-BANANA-2.1-2K",
			wantValid: true,
		},
		{
			name:      "gemini-3.1-flash-image still works",
			model:     "gemini-3.1-flash-image",
			wantValid: true,
		},
		{
			name:      "invalid gemini model should fail",
			model:     "gemini-text-only",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test isImageGenerationModel
			got := isImageGenerationModel(tt.model)
			if got != tt.wantValid {
				t.Errorf("isImageGenerationModel(%q) = %v, want %v", tt.model, got, tt.wantValid)
			}

			// Test isGeminiImageStudioModel
			got = isGeminiImageStudioModel(tt.model)
			if got != tt.wantValid {
				t.Errorf("isGeminiImageStudioModel(%q) = %v, want %v", tt.model, got, tt.wantValid)
			}

			// Test capability resolution
			capability, ok := resolveGeminiImageStudioCapability(tt.model)
			if ok != tt.wantValid {
				t.Errorf("resolveGeminiImageStudioCapability(%q) ok = %v, want %v", tt.model, ok, tt.wantValid)
			}
			if tt.wantValid && capability.Platform != PlatformGemini {
				t.Errorf("resolveGeminiImageStudioCapability(%q).Platform = %q, want %q", tt.model, capability.Platform, PlatformGemini)
			}

			// Test OpenAI images validation
			err := validateOpenAIImagesModel(tt.model)
			if tt.wantValid && err != nil {
				t.Errorf("validateOpenAIImagesModel(%q) = %v, want nil", tt.model, err)
			}
			if !tt.wantValid && err == nil {
				t.Errorf("validateOpenAIImagesModel(%q) = nil, want error", tt.model)
			}
		})
	}
}
