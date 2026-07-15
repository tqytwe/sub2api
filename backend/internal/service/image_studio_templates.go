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
	ID              string                      `json:"id"`
	Label           ImageStudioLocalizedText    `json:"label"`
	Description     ImageStudioLocalizedText    `json:"description"`
	Defaults        ImageStudioTemplateDefaults `json:"defaults"`
	PromptTemplate  string                      `json:"-"`
	ComplianceHints []string                    `json:"compliance_hints,omitempty"`
	PreviewEmoji    string                      `json:"preview_emoji,omitempty"`
	PreviewURL      string                      `json:"preview_url,omitempty"`
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
						Description: ImageStudioLocalizedText{
							Zh: "纯白背景、主体居中，适合商品主图",
							En: "Centered product on white, ready for marketplace listings",
						},
						Defaults: ImageStudioTemplateDefaults{
							Size:  "1024x1024",
							Count: 4,
						},
						PromptTemplate:  "Professional product photo, {{subject}}, pure white background RGB 255, centered, soft studio lighting, 85% frame fill, no text, no watermark",
						ComplianceHints: []string{"亚马逊主图需纯白底", "主体占画面约 85%"},
						PreviewEmoji:    "📦",
						PreviewURL:      "/image-studio/templates/ecom-white-bg.webp",
					},
				},
			},
			{
				ID:    "social",
				Label: ImageStudioLocalizedText{Zh: "小红书封面", En: "Social cover"},
				Templates: []ImageStudioTemplate{
					{
						ID:    "xhs-cover",
						Label: ImageStudioLocalizedText{Zh: "竖版封面 2:3", En: "Portrait cover 2:3"},
						Description: ImageStudioLocalizedText{
							Zh: "竖版构图、标题留白，适合社交内容封面",
							En: "Portrait composition with title-safe space for social covers",
						},
						Defaults: ImageStudioTemplateDefaults{
							Size:  "1024x1536",
							Count: 4,
						},
						PromptTemplate:  "Social media cover design for {{subject}}, bold readable title area at top, clean modern layout, high contrast, no watermark",
						ComplianceHints: []string{"标题区留白", "手机缩略图可读"},
						PreviewEmoji:    "📱",
						PreviewURL:      "/image-studio/templates/xhs-cover.webp",
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
						Description: ImageStudioLocalizedText{
							Zh: "不限制题材，用自然语言自由描述画面",
							En: "Open-ended creation from a natural-language description",
						},
						Defaults: ImageStudioTemplateDefaults{
							Size:  "1024x1024",
							Count: 1,
						},
						PromptTemplate: "High quality creative illustration of {{subject}}, detailed, aesthetic composition",
						PreviewEmoji:   "✨",
						PreviewURL:     "/image-studio/templates/free-create.webp",
					},
				},
			},
		},
	}
}
