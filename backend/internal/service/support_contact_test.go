//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSupportContactConfigForStorageRejectsTooManyPrimaryContacts(t *testing.T) {
	_, _, err := NormalizeSupportContactConfigForStorage(SupportContactConfig{
		Contacts: []SupportContactMethod{
			{ID: "wechat", Type: "wechat", Value: "wx", Primary: true, Enabled: true},
			{ID: "qq", Type: "qq", Value: "123", Primary: true, Enabled: true},
			{ID: "telegram", Type: "telegram", Value: "@support", Primary: true, Enabled: true},
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "at most 2 primary")
}

func TestNormalizeSupportContactConfigForStorageRejectsUnsafeQRImage(t *testing.T) {
	_, _, err := NormalizeSupportContactConfigForStorage(SupportContactConfig{
		Contacts: []SupportContactMethod{
			{ID: "wechat", Type: "wechat", Value: "wx", QRImage: "javascript:alert(1)", Enabled: true},
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "qr_image")
}

func TestNormalizeSupportContactConfigForStorageRejectsOversizedQRImage(t *testing.T) {
	_, _, err := NormalizeSupportContactConfigForStorage(SupportContactConfig{
		Contacts: []SupportContactMethod{
			{ID: "wechat", Type: "wechat", Value: "wx", QRImage: "data:image/png;base64," + strings.Repeat("a", maxSupportContactImageLength), Enabled: true},
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "too large")
}

func TestRewritePublicSupportContactQRImages_ReplacesDataURLWithPublicAssetURL(t *testing.T) {
	config := SupportContactConfig{
		Contacts: []SupportContactMethod{
			{ID: "wechat main", Type: "wechat", Value: "wx", QRImage: "data:image/png;base64,aGk=", Enabled: true},
			{ID: "docs", Type: "docs", URL: "/docs", QRImage: "/uploads/docs.png", Enabled: true},
		},
	}

	rewritten := RewritePublicSupportContactQRImages(config)

	require.Equal(t, "/api/v1/settings/public/support-contact/qr/wechat%20main", rewritten.Contacts[0].QRImage)
	require.Equal(t, "/uploads/docs.png", rewritten.Contacts[1].QRImage)
	require.Equal(t, "data:image/png;base64,aGk=", config.Contacts[0].QRImage)
}

func TestSettingService_GetPublicSupportContactQRCode_DecodesPublicDataURL(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeySupportContactConfig: `{
				"contacts":[
					{"id":"wechat-main","type":"wechat","value":"wx","qr_image":"data:image/png;base64,aGk=","enabled":true}
				]
			}`,
		},
	}, &config.Config{})

	asset, err := svc.GetPublicSupportContactQRCode(context.Background(), "wechat-main")

	require.NoError(t, err)
	require.Equal(t, "image/png", asset.ContentType)
	require.Equal(t, []byte("hi"), asset.Data)
	require.NotEmpty(t, asset.ETag)
}
