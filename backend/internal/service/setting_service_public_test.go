//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type settingPublicRepoStub struct {
	values            map[string]string
	err               error
	getMultipleErr    error
	getMultipleErrKey map[string]error
	getMultipleCalls  [][]string
}

func (s *settingPublicRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingPublicRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", errors.New("setting not found")
}

func (s *settingPublicRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingPublicRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	s.getMultipleCalls = append(s.getMultipleCalls, append([]string(nil), keys...))
	if s.err != nil {
		return nil, s.err
	}
	if s.getMultipleErr != nil {
		return nil, s.getMultipleErr
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if err := s.getMultipleErrKey[key]; err != nil {
			return nil, err
		}
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *settingPublicRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingPublicRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingPublicRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_GetPublicSettings_ExposesRegistrationEmailSuffixWhitelist(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyRegistrationEnabled:              "true",
			SettingKeyEmailVerifyEnabled:               "true",
			SettingKeyRegistrationEmailSuffixWhitelist: `["@EXAMPLE.com"," @foo.bar ","*.EDU.CN","@invalid_domain",""]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"@example.com", "@foo.bar", "*.edu.cn"}, settings.RegistrationEmailSuffixWhitelist)
}

func TestSettingService_GetPublicSettings_ExposesRegistrationEmailDomainQuotaEnabled(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyRegistrationEmailDomainQuotaEnabled: "true",
		},
	}

	settings, err := NewSettingService(repo, &config.Config{}).GetPublicSettings(context.Background())

	require.NoError(t, err)
	require.True(t, settings.RegistrationEmailDomainQuotaEnabled)
}

func TestSettingService_GetPublicSettings_ExposesTablePreferences(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyTableDefaultPageSize: "50",
			SettingKeyTablePageSizeOptions: "[20,50,100]",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 50, settings.TableDefaultPageSize)
	require.Equal(t, []int{20, 50, 100}, settings.TablePageSizeOptions)
}

func TestSettingService_GetPublicSettings_ExposesCompactHomeEnabled(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyCompactHomeEnabled: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())

	require.NoError(t, err)
	require.True(t, settings.CompactHomeEnabled)

	missingSettings, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).
		GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, missingSettings.CompactHomeEnabled)
}

func TestSettingService_ChannelMonitorHideThroughputDefaultsToPrivate(t *testing.T) {
	missing := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.True(t, missing.HideThroughput)
	public, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.ChannelMonitorHideThroughput)

	for _, value := range []string{"false", "0", "off", "disabled"} {
		runtime := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyChannelMonitorHideThroughput: value,
		}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
		require.False(t, runtime.HideThroughput, "value=%q", value)
	}
}

func TestSettingService_ChannelMonitorShowQuotaFailsClosed(t *testing.T) {
	// 缺省（迁移插入 'false' / 老库无行）一律不展示。
	missingRuntime := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.False(t, missingRuntime.ShowQuota)
	missingPublic, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).
		GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, missingPublic.ChannelMonitorShowQuota)

	// 仅字面 "true" 视为开启；其余值（含异常值）fail-closed。
	runtime := NewSettingService(&settingPublicRepoStub{values: map[string]string{
		SettingKeyChannelMonitorShowQuota: "true",
	}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.True(t, runtime.ShowQuota)

	for _, value := range []string{"false", "TRUE", "1", "yes", "on", "garbage"} {
		rt := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyChannelMonitorShowQuota: value,
		}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
		require.False(t, rt.ShowQuota, "value=%q", value)
	}
}

func TestSettingService_ChannelMonitorHideUserRankingDefaultsToVisible(t *testing.T) {
	missing := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.False(t, missing.HideUserRanking)
	public, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, public.ChannelMonitorHideUserRanking)

	for _, value := range []string{"true", "1", "on", "enabled"} {
		runtime := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyChannelMonitorHideUserRanking: value,
		}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
		require.True(t, runtime.HideUserRanking, "value=%q", value)
	}
}

func TestSettingService_GetPublicSettings_ExposesForceEmailOnThirdPartySignup(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyForceEmailOnThirdPartySignup: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.ForceEmailOnThirdPartySignup)
}

func TestSettingService_GetPublicSettings_ExposesAllowUserViewErrorRequests(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyAllowUserViewErrorRequests: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.AllowUserViewErrorRequests)
}

func TestSettingService_GetPublicSettings_ExposesMarketplaceEnabled(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		present bool
		want    bool
	}{
		{name: "missing", want: false},
		{name: "false", raw: "false", present: true, want: false},
		{name: "invalid", raw: "TRUE", present: true, want: false},
		{name: "true", raw: "true", present: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[string]string{}
			if tt.present {
				values[SettingKeyMarketplaceEnabled] = tt.raw
			}
			repo := &settingPublicRepoStub{
				values: values,
			}
			svc := NewSettingService(repo, &config.Config{})

			settings, err := svc.GetPublicSettings(context.Background())

			require.NoError(t, err)
			require.Equal(t, tt.want, settings.MarketplaceEnabled)
			require.Len(t, repo.getMultipleCalls, 1)
			require.Contains(t, repo.getMultipleCalls[0], SettingKeyMarketplaceEnabled)
		})
	}
}

func TestSettingService_GetPublicSettings_MarketplaceBulkReadErrorPropagatesWithoutRetry(t *testing.T) {
	repoErr := errors.New("database unavailable")
	repo := &settingPublicRepoStub{
		values:         map[string]string{},
		getMultipleErr: repoErr,
	}
	svc := NewSettingService(repo, &config.Config{})

	_, err := svc.GetPublicSettings(context.Background())

	require.ErrorIs(t, err, repoErr)
	require.Len(t, repo.getMultipleCalls, 1)
	require.Contains(t, repo.getMultipleCalls[0], SettingKeyMarketplaceEnabled)
}

func TestSettingService_GetPublicSettings_MarketplaceReadCancellationPropagates(t *testing.T) {
	repo := &settingPublicRepoStub{
		values:         map[string]string{},
		getMultipleErr: context.Canceled,
	}
	svc := NewSettingService(repo, &config.Config{})

	_, err := svc.GetPublicSettings(context.Background())

	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, repo.getMultipleCalls, 1)
	require.Contains(t, repo.getMultipleCalls[0], SettingKeyMarketplaceEnabled)
}

func TestSettingService_GetPublicSettingsForInjection_ExposesMarketplaceEnabled(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "false", raw: "false", want: false},
		{name: "true", raw: "true", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSettingService(&settingPublicRepoStub{
				values: map[string]string{SettingKeyMarketplaceEnabled: tt.raw},
			}, &config.Config{})

			payload, err := svc.GetPublicSettingsForInjection(context.Background())

			require.NoError(t, err)
			injection, ok := payload.(*PublicSettingsInjectionPayload)
			require.True(t, ok)
			require.Equal(t, tt.want, injection.MarketplaceEnabled)
		})
	}
}

func TestSettingService_GetPublicSettingsForInjection_MarketplaceBulkReadErrorPropagates(t *testing.T) {
	repoErr := errors.New("database unavailable")
	svc := NewSettingService(&settingPublicRepoStub{
		values:         map[string]string{},
		getMultipleErr: repoErr,
	}, &config.Config{})

	_, err := svc.GetPublicSettingsForInjection(context.Background())

	require.ErrorIs(t, err, repoErr)
}

func TestSettingService_GetPublicSettings_ExposesWeChatOAuthModeCapabilities(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyWeChatConnectEnabled:             "true",
			SettingKeyWeChatConnectAppID:               "wx-mp-app",
			SettingKeyWeChatConnectAppSecret:           "wx-mp-secret",
			SettingKeyWeChatConnectMode:                "mp",
			SettingKeyWeChatConnectScopes:              "snsapi_base",
			SettingKeyWeChatConnectOpenEnabled:         "true",
			SettingKeyWeChatConnectMPEnabled:           "true",
			SettingKeyWeChatConnectRedirectURL:         "https://api.example.com/api/v1/auth/oauth/wechat/callback",
			SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
		},
	}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.WeChatOAuthEnabled)
	require.True(t, settings.WeChatOAuthOpenEnabled)
	require.True(t, settings.WeChatOAuthMPEnabled)
}

func TestSettingService_GetPublicSettings_DoesNotExposeMobileOnlyWeChatAsWebOAuthAvailable(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyWeChatConnectEnabled:             "true",
			SettingKeyWeChatConnectMobileEnabled:       "true",
			SettingKeyWeChatConnectMode:                "mobile",
			SettingKeyWeChatConnectMobileAppID:         "wx-mobile-app",
			SettingKeyWeChatConnectMobileAppSecret:     "wx-mobile-secret",
			SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
		},
	}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.WeChatOAuthEnabled)
	require.False(t, settings.WeChatOAuthOpenEnabled)
	require.False(t, settings.WeChatOAuthMPEnabled)
	require.True(t, settings.WeChatOAuthMobileEnabled)
}

func TestSettingService_GetPublicSettings_FallsBackToConfigForWeChatOAuthCapabilities(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			OpenEnabled:         true,
			OpenAppID:           "wx-open-config",
			OpenAppSecret:       "wx-open-secret",
			FrontendRedirectURL: "/auth/wechat/config-callback",
		},
	})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.WeChatOAuthEnabled)
	require.True(t, settings.WeChatOAuthOpenEnabled)
	require.False(t, settings.WeChatOAuthMPEnabled)
	require.False(t, settings.WeChatOAuthMobileEnabled)
}

func TestSettingService_GetPublicSettings_ExposesSanitizedSupportContact(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeySupportContactConfig: `{
				"title":"联系客服",
				"subtitle":"登录、充值或 API 问题都可以联系人工客服",
				"contacts":[
					{"id":"wechat-main","type":"wechat","label":"微信客服","value":"tqytwemx","copy_value":"tqytwemx","qr_image":"/uploads/wechat.png","primary":true,"enabled":true,"sort_order":2},
					{"id":"disabled-qq","type":"qq","label":"QQ客服","value":"1570539180","enabled":false,"sort_order":1}
				]
			}`,
		},
	}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "联系客服", settings.SupportContact.Title)
	require.Len(t, settings.SupportContact.Contacts, 1)
	require.Equal(t, "wechat-main", settings.SupportContact.Contacts[0].ID)
	require.Equal(t, "wechat", settings.SupportContact.Contacts[0].Type)
	require.Equal(t, "/uploads/wechat.png", settings.SupportContact.Contacts[0].QRImage)
}

func TestSettingService_GetPublicSettingsForInjection_RewritesSupportContactDataURLQR(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeySupportContactConfig: `{
				"contacts":[
					{"id":"wechat-main","type":"wechat","value":"wx","qr_image":"data:image/png;base64,aGk=","enabled":true}
				]
			}`,
		},
	}, &config.Config{})

	payload, err := svc.GetPublicSettingsForInjection(context.Background())

	require.NoError(t, err)
	injection, ok := payload.(*PublicSettingsInjectionPayload)
	require.True(t, ok)
	require.Len(t, injection.SupportContact.Contacts, 1)
	require.Equal(t, "/api/v1/settings/public/support-contact/qr/wechat-main", injection.SupportContact.Contacts[0].QRImage)
}

func TestSettingService_GetPublicSettings_BuildsSupportContactFromLegacyFields(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyContactInfo: "1570539180 微信：tqytwemx",
			SettingKeyDocURL:      "https://docs.example.com",
		},
	}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "联系客服", settings.SupportContact.Title)
	require.Len(t, settings.SupportContact.Contacts, 2)
	require.Equal(t, "legacy-contact", settings.SupportContact.Contacts[0].ID)
	require.Equal(t, "1570539180 微信：tqytwemx", settings.SupportContact.Contacts[0].CopyValue)
	require.Equal(t, "legacy-docs", settings.SupportContact.Contacts[1].ID)
	require.Equal(t, "https://docs.example.com", settings.SupportContact.Contacts[1].URL)
}
