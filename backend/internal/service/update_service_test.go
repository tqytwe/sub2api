package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type sourcePolicyCache struct{ data string }

func (c *sourcePolicyCache) GetUpdateInfo(context.Context) (string, error) { return c.data, nil }
func (c *sourcePolicyCache) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	c.data = data
	return nil
}

type sourcePolicyClient struct {
	release *GitHubRelease
	repo    string
	calls   int
	err     error
}

func (c *sourcePolicyClient) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	c.repo = repo
	c.calls++
	return c.release, c.err
}
func (c *sourcePolicyClient) FetchRecentReleases(context.Context, string, int) ([]*GitHubRelease, error) {
	panic("unverified rollback release query")
}
func (c *sourcePolicyClient) DownloadFile(context.Context, string, string, int64) error {
	panic("unverified binary download")
}
func (c *sourcePolicyClient) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("unverified checksum download")
}
func policyRelease() *GitHubRelease {
	return &GitHubRelease{TagName: "v2.10.3", HTMLURL: "https://github.com/ranxi2001/sub2api/releases/tag/v2.10.3", Assets: []GitHubAsset{{Name: "sub2api_linux_amd64.tar.gz", BrowserDownloadURL: "https://github.com/ranxi2001/sub2api/releases/download/v2.10.3/sub2api_linux_amd64.tar.gz"}}}
}

func TestUpdateSourcePolicyRejectsAllBinaryMutation(t *testing.T) {
	for _, build := range []string{"source", "release", ""} {
		t.Run(build, func(t *testing.T) {
			client := &sourcePolicyClient{release: policyRelease()}
			svc := NewUpdateService(&sourcePolicyCache{}, client, "0.2.14", build)
			for _, err := range []error{svc.PerformUpdate(context.Background()), svc.RollbackToVersion(context.Background(), "2.10.2"), svc.Rollback()} {
				if err == nil || !strings.Contains(err.Error(), "source") {
					t.Fatalf("expected source deployment refusal, got %v", err)
				}
			}
			versions, err := svc.ListRollbackVersions(context.Background())
			if err != nil || len(versions) != 0 {
				t.Fatalf("unverified rollbacks: %v %v", versions, err)
			}
			if client.calls != 0 {
				t.Fatal("mutation must fail before network or filesystem access")
			}
		})
	}
}

func TestUpdateSourcePolicyCheckAndCache(t *testing.T) {
	cache := &sourcePolicyCache{data: fmt.Sprintf(`{"latest":"99.0.0","timestamp":%d,"release_info":{"html_url":"https://github.com/Wei-Shaw/sub2api"}}`, time.Now().Unix())}
	client := &sourcePolicyClient{release: policyRelease()}
	svc := NewUpdateService(cache, client, "0.2.14", "release")
	info, err := svc.CheckUpdate(context.Background(), false)
	if err != nil || info.LatestVersion != "2.10.3" || info.Cached || client.repo != "ranxi2001/sub2api" {
		t.Fatalf("old cache/source reused: %+v %v %s", info, err, client.repo)
	}
	if len(info.ReleaseInfo.Assets) != 0 {
		t.Fatal("upstream assets must never be offered as fork updates")
	}
	body, _ := json.Marshal(info)
	var response map[string]any
	_ = json.Unmarshal(body, &response)
	if response["install_supported"] != false || response["install_repository"] != "tqytwe/sub2api" {
		t.Fatalf("missing installation policy: %s", body)
	}
	info, err = svc.CheckUpdate(context.Background(), false)
	if err != nil || !info.Cached || client.calls != 1 {
		t.Fatalf("cache should be reusable only for matching policy: %+v %v", info, err)
	}
	var cached map[string]any
	_ = json.Unmarshal([]byte(cache.data), &cached)
	cached["source_identity"] = "Wei-Shaw/sub2api"
	changed, _ := json.Marshal(cached)
	cache.data = string(changed)
	client.err = fmt.Errorf("offline")
	info, err = svc.CheckUpdate(context.Background(), true)
	if err != nil || info.Cached || info.HasUpdate || info.Warning == "" {
		t.Fatalf("wrong-source cache accepted on failure: %+v %v", info, err)
	}
}

func TestUpdateSourcePolicyRejectsUnofficialRelease(t *testing.T) {
	for _, change := range []func(*GitHubRelease){func(r *GitHubRelease) { r.Draft = true }, func(r *GitHubRelease) { r.Prerelease = true }, func(r *GitHubRelease) { r.HTMLURL = "https://github.com/Wei-Shaw/sub2api/releases/tag/v2.10.3" }, func(r *GitHubRelease) { r.TagName = "v2.10.3-rc1" }} {
		release := policyRelease()
		change(release)
		svc := NewUpdateService(&sourcePolicyCache{}, &sourcePolicyClient{release: release}, "0.2.14", "release")
		info, err := svc.CheckUpdate(context.Background(), true)
		if err != nil || info.HasUpdate || info.ReleaseInfo != nil || info.Warning == "" {
			t.Fatalf("unofficial release accepted: %+v %v", info, err)
		}
	}
}

func TestUpdateSourcePolicyComparesUpstreamPinNotForkVersion(t *testing.T) {
	for _, forkVersion := range []string{"0.2.14", "999.0.0", "dev"} {
		release := policyRelease()
		client := &sourcePolicyClient{release: release}
		svc := NewUpdateService(&sourcePolicyCache{}, client, forkVersion, "release")
		info, err := svc.CheckUpdate(context.Background(), true)
		if err != nil || info.HasUpdate {
			t.Fatalf("the pinned upstream release is not a new fork update: %+v %v", info, err)
		}
		release.TagName = "v2.11.0"
		release.HTMLURL = "https://github.com/ranxi2001/sub2api/releases/tag/v2.11.0"
		info, err = svc.CheckUpdate(context.Background(), true)
		if err != nil || !info.HasUpdate {
			t.Fatalf("new upstream should be compared to review pin: %+v %v", info, err)
		}
	}
}
