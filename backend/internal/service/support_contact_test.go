//go:build unit

package service

import (
	"strings"
	"testing"

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
