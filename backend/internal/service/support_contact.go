package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	defaultSupportContactTitle    = "联系客服"
	defaultSupportContactSubtitle = "登录、注册、充值、API 或模型调用问题都可以联系人工客服"
	maxSupportContacts            = 20
	maxPrimarySupportContacts     = 2
	maxSupportContactTextLength   = 240
	maxSupportContactDescLength   = 360
	maxSupportContactImageLength  = 460 * 1024
	publicSupportContactQRPrefix  = "/api/v1/settings/public/support-contact/qr/"
)

var ErrSupportContactQRCodeNotFound = errors.New("support contact QR image not found")

var allowedSupportContactTypes = map[string]bool{
	"wechat":   true,
	"qq":       true,
	"telegram": true,
	"email":    true,
	"docs":     true,
	"custom":   true,
}

type SupportContactConfig struct {
	Title    string                 `json:"title"`
	Subtitle string                 `json:"subtitle"`
	Contacts []SupportContactMethod `json:"contacts"`
}

type SupportContactMethod struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	CopyValue   string `json:"copy_value"`
	URL         string `json:"url"`
	QRImage     string `json:"qr_image"`
	Description string `json:"description"`
	Primary     bool   `json:"primary"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

type PublicSupportContactQRCode struct {
	ContentType string
	Data        []byte
	ETag        string
}

func NormalizeSupportContactConfigForStorage(config SupportContactConfig) (SupportContactConfig, string, error) {
	normalized, err := normalizeSupportContactConfig(config, true, true)
	if err != nil {
		return SupportContactConfig{}, "", err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return SupportContactConfig{}, "", fmt.Errorf("marshal support contact config: %w", err)
	}
	return normalized, string(raw), nil
}

func BuildAdminSupportContactConfig(raw, legacyContactInfo, legacyDocURL string) SupportContactConfig {
	config := parseSupportContactConfig(raw)
	normalized, err := normalizeSupportContactConfig(config, true, false)
	if err != nil {
		normalized = SupportContactConfig{}
	}
	return withLegacySupportContactFallback(normalized, legacyContactInfo, legacyDocURL)
}

func BuildPublicSupportContactConfig(raw, legacyContactInfo, legacyDocURL string) SupportContactConfig {
	config := parseSupportContactConfig(raw)
	normalized, err := normalizeSupportContactConfig(config, false, false)
	if err != nil {
		normalized = SupportContactConfig{}
	}
	return withLegacySupportContactFallback(normalized, legacyContactInfo, legacyDocURL)
}

func RewritePublicSupportContactQRImages(config SupportContactConfig) SupportContactConfig {
	out := config
	out.Contacts = append([]SupportContactMethod(nil), config.Contacts...)
	for i := range out.Contacts {
		if isSupportContactDataImage(out.Contacts[i].QRImage) {
			out.Contacts[i].QRImage = publicSupportContactQRPrefix + url.PathEscape(out.Contacts[i].ID)
		}
	}
	return out
}

func (s *SettingService) GetPublicSupportContactQRCode(ctx context.Context, id string) (PublicSupportContactQRCode, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PublicSupportContactQRCode{}, ErrSupportContactQRCodeNotFound
	}
	settings, err := s.GetPublicSettings(ctx)
	if err != nil {
		return PublicSupportContactQRCode{}, err
	}
	for _, contact := range settings.SupportContact.Contacts {
		if contact.ID != id {
			continue
		}
		contentType, data, ok := decodeSupportContactDataImage(contact.QRImage)
		if !ok {
			return PublicSupportContactQRCode{}, ErrSupportContactQRCodeNotFound
		}
		sum := sha256.Sum256(data)
		return PublicSupportContactQRCode{
			ContentType: contentType,
			Data:        data,
			ETag:        fmt.Sprintf(`"%x"`, sum[:]),
		}, nil
	}
	return PublicSupportContactQRCode{}, ErrSupportContactQRCodeNotFound
}

func parseSupportContactConfig(raw string) SupportContactConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SupportContactConfig{}
	}
	var config SupportContactConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return SupportContactConfig{}
	}
	return config
}

func normalizeSupportContactConfig(config SupportContactConfig, includeDisabled bool, rejectInvalid bool) (SupportContactConfig, error) {
	out := SupportContactConfig{
		Title:    trimSupportContactText(config.Title, maxSupportContactTextLength),
		Subtitle: trimSupportContactText(config.Subtitle, maxSupportContactDescLength),
		Contacts: []SupportContactMethod{},
	}
	if out.Title == "" {
		out.Title = defaultSupportContactTitle
	}
	if out.Subtitle == "" {
		out.Subtitle = defaultSupportContactSubtitle
	}
	if len(config.Contacts) > maxSupportContacts {
		if rejectInvalid {
			return SupportContactConfig{}, fmt.Errorf("support_contact.contacts cannot exceed %d", maxSupportContacts)
		}
		config.Contacts = config.Contacts[:maxSupportContacts]
	}

	type indexedContact struct {
		index   int
		contact SupportContactMethod
	}
	indexed := make([]indexedContact, 0, len(config.Contacts))
	primaryCount := 0
	for i, contact := range config.Contacts {
		normalized, err := normalizeSupportContactMethod(contact, i+1, rejectInvalid)
		if err != nil {
			if rejectInvalid {
				return SupportContactConfig{}, err
			}
			continue
		}
		if !includeDisabled && !normalized.Enabled {
			continue
		}
		if normalized.Primary {
			primaryCount++
			if primaryCount > maxPrimarySupportContacts {
				if rejectInvalid {
					return SupportContactConfig{}, fmt.Errorf("support_contact supports at most %d primary contacts", maxPrimarySupportContacts)
				}
				normalized.Primary = false
			}
		}
		indexed = append(indexed, indexedContact{index: i, contact: normalized})
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		left := indexed[i]
		right := indexed[j]
		if left.contact.SortOrder == right.contact.SortOrder {
			return left.index < right.index
		}
		return left.contact.SortOrder < right.contact.SortOrder
	})
	for _, item := range indexed {
		out.Contacts = append(out.Contacts, item.contact)
	}
	return out, nil
}

func normalizeSupportContactMethod(contact SupportContactMethod, position int, rejectInvalid bool) (SupportContactMethod, error) {
	contact.ID = trimSupportContactText(contact.ID, maxSupportContactTextLength)
	contact.Type = strings.ToLower(trimSupportContactText(contact.Type, 32))
	if contact.Type == "" {
		contact.Type = "custom"
	}
	if !allowedSupportContactTypes[contact.Type] {
		return SupportContactMethod{}, fmt.Errorf("support_contact.contacts[%d].type is unsupported", position-1)
	}
	if contact.ID == "" {
		contact.ID = fmt.Sprintf("%s-%d", contact.Type, position)
	}
	contact.Label = trimSupportContactText(contact.Label, maxSupportContactTextLength)
	if contact.Label == "" {
		contact.Label = defaultSupportContactLabel(contact.Type)
	}
	contact.Value = trimSupportContactText(contact.Value, maxSupportContactTextLength)
	contact.CopyValue = trimSupportContactText(contact.CopyValue, maxSupportContactTextLength)
	contact.Description = trimSupportContactText(contact.Description, maxSupportContactDescLength)
	contact.URL = strings.TrimSpace(contact.URL)
	if contact.URL != "" && !isSafeSupportContactURL(contact.URL) {
		if rejectInvalid {
			return SupportContactMethod{}, fmt.Errorf("support_contact.contacts[%d].url must be http(s), mailto, or a site-relative path", position-1)
		}
		contact.URL = ""
	}
	contact.QRImage = strings.TrimSpace(contact.QRImage)
	if contact.QRImage != "" && !isSafeSupportContactImage(contact.QRImage) {
		if rejectInvalid {
			return SupportContactMethod{}, fmt.Errorf("support_contact.contacts[%d].qr_image must be https, site-relative, or a safe image data URL", position-1)
		}
		contact.QRImage = ""
	}
	if contact.QRImage != "" && len(contact.QRImage) > maxSupportContactImageLength {
		if rejectInvalid {
			return SupportContactMethod{}, fmt.Errorf("support_contact.contacts[%d].qr_image is too large", position-1)
		}
		contact.QRImage = ""
	}
	if contact.SortOrder <= 0 {
		contact.SortOrder = position
	}
	if contact.Value == "" && contact.CopyValue == "" && contact.URL == "" && contact.QRImage == "" {
		return SupportContactMethod{}, fmt.Errorf("support_contact.contacts[%d] must include value, copy_value, url, or qr_image", position-1)
	}
	return contact, nil
}

func withLegacySupportContactFallback(config SupportContactConfig, legacyContactInfo, legacyDocURL string) SupportContactConfig {
	if len(config.Contacts) > 0 {
		return config
	}
	config.Title = defaultSupportContactTitle
	config.Subtitle = defaultSupportContactSubtitle
	legacyContactInfo = strings.TrimSpace(legacyContactInfo)
	legacyDocURL = strings.TrimSpace(legacyDocURL)
	if legacyContactInfo != "" {
		config.Contacts = append(config.Contacts, SupportContactMethod{
			ID:        "legacy-contact",
			Type:      "custom",
			Label:     "客服联系方式",
			Value:     trimSupportContactText(legacyContactInfo, maxSupportContactTextLength),
			CopyValue: trimSupportContactText(legacyContactInfo, maxSupportContactTextLength),
			Enabled:   true,
			SortOrder: 1,
		})
	}
	if legacyDocURL != "" && isSafeSupportContactURL(legacyDocURL) {
		config.Contacts = append(config.Contacts, SupportContactMethod{
			ID:        "legacy-docs",
			Type:      "docs",
			Label:     "文档链接",
			Value:     legacyDocURL,
			URL:       legacyDocURL,
			Enabled:   true,
			SortOrder: 2,
		})
	}
	return config
}

func trimSupportContactText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func defaultSupportContactLabel(contactType string) string {
	switch contactType {
	case "wechat":
		return "微信客服"
	case "qq":
		return "QQ 客服"
	case "telegram":
		return "Telegram"
	case "email":
		return "邮箱"
	case "docs":
		return "文档"
	default:
		return "客服"
	}
}

func isSafeSupportContactURL(raw string) bool {
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return true
	default:
		return false
	}
}

func isSafeSupportContactImage(raw string) bool {
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return true
	}
	if strings.HasPrefix(raw, "data:") {
		lower := strings.ToLower(raw)
		allowed := strings.HasPrefix(lower, "data:image/png;base64,") ||
			strings.HasPrefix(lower, "data:image/jpeg;base64,") ||
			strings.HasPrefix(lower, "data:image/jpg;base64,") ||
			strings.HasPrefix(lower, "data:image/webp;base64,") ||
			strings.HasPrefix(lower, "data:image/gif;base64,")
		if !allowed {
			return false
		}
		comma := strings.Index(raw, ",")
		if comma < 0 || comma == len(raw)-1 {
			return false
		}
		_, err := base64.StdEncoding.DecodeString(raw[comma+1:])
		return err == nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, "https") && u.Host != ""
}

func isSupportContactDataImage(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "data:image/png;base64,") ||
		strings.HasPrefix(lower, "data:image/jpeg;base64,") ||
		strings.HasPrefix(lower, "data:image/jpg;base64,") ||
		strings.HasPrefix(lower, "data:image/webp;base64,") ||
		strings.HasPrefix(lower, "data:image/gif;base64,")
}

func decodeSupportContactDataImage(raw string) (string, []byte, bool) {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	contentType := ""
	dataStart := 0
	for prefix, normalizedContentType := range map[string]string{
		"data:image/png;base64,":  "image/png",
		"data:image/jpeg;base64,": "image/jpeg",
		"data:image/jpg;base64,":  "image/jpeg",
		"data:image/webp;base64,": "image/webp",
		"data:image/gif;base64,":  "image/gif",
	} {
		if strings.HasPrefix(lower, prefix) {
			contentType = normalizedContentType
			dataStart = len(prefix)
			break
		}
	}
	if contentType == "" || dataStart <= 0 || dataStart >= len(trimmed) {
		return "", nil, false
	}
	data, err := base64.StdEncoding.DecodeString(trimmed[dataStart:])
	if err != nil || len(data) == 0 {
		return "", nil, false
	}
	return contentType, data, true
}
