package service

import "testing"

func TestConfigureCodexCanonicalUserAgentResolverUsesSyncedVersion(t *testing.T) {
	SetCodexCanonicalUserAgentResolver(nil)
	t.Cleanup(func() { SetCodexCanonicalUserAgentResolver(nil) })

	settingService := NewSettingService(&codexVersionSettingRepoStub{values: map[string]string{
		SettingKeyOpenAICodexClientVersionSynced: "0.154.0",
	}}, nil)

	configureCodexCanonicalUserAgentResolver(settingService)

	if got := CodexCanonicalClientVersion(); got != "0.154.0" {
		t.Fatalf("CodexCanonicalClientVersion() = %q, want %q", got, "0.154.0")
	}
	wantUA := "codex-tui/0.154.0" + codexCLIUserAgentSuffix
	if got := CodexCanonicalUserAgent(); got != wantUA {
		t.Fatalf("CodexCanonicalUserAgent() = %q, want %q", got, wantUA)
	}
}
