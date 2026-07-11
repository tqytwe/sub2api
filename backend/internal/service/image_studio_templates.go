package service

type ImageStudioLocalizedText struct {
	Zh string `json:"zh"`
	En string `json:"en"`
}

type ImageStudioTemplateDefaults struct {
	Size  string `json:"size"`
	Count int    `json:"count"`
}

type ImageStudioTemplate struct {
	ID               string                    `json:"id"`
	Label            ImageStudioLocalizedText  `json:"label"`
	Defaults         ImageStudioTemplateDefaults `json:"defaults"`
	PromptTemplate   string                    `json:"-"`
	ComplianceHints  []string                  `json:"compliance_hints,omitempty"`
	PreviewEmoji     string                    `json:"preview_emoji,omitempty"`
}

type ImageStudioIntent struct {
	ID        string                   `json:"id"`
	Label     ImageStudioLocalizedText `json:"label"`
	Templates []ImageStudioTemplate    `json:"templates"`
}

type ImageStudioCatalog struct {
	Intents []ImageStudioIntent `json:"intents"`
}

func defaultImageStudioCatalog() ImageStudioCatalog {
	return ImageStudioCatalog{
		Intents: []ImageStudioIntent{
			{
				ID:    "ecommerce",
				Label: ImageStudioLocalizedText{Zh: "电商主图", En: "E-commerce"},
				Templates: []ImageStudioTemplate{
					{
						ID:    "ecom-white-bg",
						Label: ImageStudioLocalizedText{Zh: "亚马逊白底主图", En: "Amazon white background"},
						Defaults: ImageStudioTemplateDefaults{
							Size:  "1024x1024",
							Count: 4,
						},
						PromptTemplate:  "Professional product photo, {{subject}}, pure white background RGB 255, centered, soft studio lighting, 85% frame fill, no text, no watermark",
						ComplianceHints: []string{"亚马逊主图需纯白底", "主体占画面约 85%"},
						PreviewEmoji:    "📦",
					},
				},
			},
			{
				ID:    "social",
				Label: ImageStudioLocalizedText{Zh: "小红书封面", En: "Social cover"},
				Templates: []ImageStudioTemplate{
					{
						ID:    "xhs-cover",
						Label: ImageStudioLocalizedText{Zh: "竖版封面 3:4", En: "Portrait cover 3:4"},
						Defaults: ImageStudioTemplateDefaults{
							Size:  "1024x1536",
							Count: 4,
						},
						PromptTemplate:  "Social media cover design for {{subject}}, bold readable title area at top, clean modern layout, high contrast, no watermark",
						ComplianceHints: []string{"标题区留白", "手机缩略图可读"},
						PreviewEmoji:    "📱",
					},
				},
			},
			{
				ID:    "free",
				Label: ImageStudioLocalizedText{Zh: "自由创作", En: "Free create"},
				Templates: []ImageStudioTemplate{
					{
						ID:    "free-create",
						Label: ImageStudioLocalizedText{Zh: "通用创意", En: "General creative"},
						Defaults: ImageStudioTemplateDefaults{
							Size:  "1024x1024",
							Count: 1,
						},
						PromptTemplate: "High quality creative illustration of {{subject}}, detailed, aesthetic composition",
						PreviewEmoji:   "✨",
					},
				},
			},
		},
	}
}
