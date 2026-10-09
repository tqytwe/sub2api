//go:build unit

package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGroupAllowedModels(t *testing.T) {
	require.Nil(t, NormalizeGroupAllowedModels(nil))
	require.Nil(t, NormalizeGroupAllowedModels([]string{" ", ""}))
	require.Equal(t,
		[]string{"gpt-5.5", "claude-*"},
		NormalizeGroupAllowedModels([]string{" gpt-5.5 ", "", "claude-*", "gpt-5.5"}),
		"去掉空白、空项和重复项，保留填写顺序",
	)
}

func TestValidateGroupAllowedModels(t *testing.T) {
	require.NoError(t, ValidateGroupAllowedModels(nil))
	require.NoError(t, ValidateGroupAllowedModels(map[int64][]string{1: {"gpt-5.5"}, 2: nil}))

	require.Error(t, ValidateGroupAllowedModels(map[int64][]string{0: {"gpt-5.5"}}), "分组 ID 必须为正数")

	tooMany := make([]string, 0, maxGroupAllowedModels+1)
	for i := 0; i <= maxGroupAllowedModels; i++ {
		tooMany = append(tooMany, fmt.Sprintf("model-%d", i))
	}
	require.Error(t, ValidateGroupAllowedModels(map[int64][]string{1: tooMany}))

	require.Error(t, ValidateGroupAllowedModels(map[int64][]string{1: {strings.Repeat("m", maxGroupAllowedModelLength+1)}}))
}

func TestAccountIsModelAllowedInGroup(t *testing.T) {
	groupA, groupB := int64(1), int64(2)
	account := &Account{
		Platform: PlatformAnthropic,
		AccountGroups: []AccountGroup{
			{GroupID: groupA},
			{GroupID: groupB, AllowedModels: []string{"claude-sonnet-4-5-20250929", "claude-haiku-*"}},
		},
	}

	cases := []struct {
		name    string
		groupID *int64
		model   string
		want    bool
	}{
		{"没有分组上下文时不限制", nil, "claude-opus-4-8", true},
		{"分组没有设置限制时不限制", &groupA, "claude-opus-4-8", true},
		{"未绑定的分组不限制", func() *int64 { id := int64(3); return &id }(), "claude-opus-4-8", true},
		{"清单内的模型放行", &groupB, "claude-sonnet-4-5-20250929", true},
		{"末尾通配匹配", &groupB, "claude-haiku-4-5-20251001", true},
		{"客户端短别名按完整 ID 匹配", &groupB, "claude-sonnet-4-5", true},
		{"清单外的模型拒绝", &groupB, "claude-opus-4-8", false},
		{"没有指定模型时不限制", &groupB, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, account.IsModelAllowedInGroup(tc.groupID, tc.model))
		})
	}

	// 分组限制只能在账号自身支持的范围内收窄
	mapped := &Account{
		Platform:      PlatformAnthropic,
		Type:          AccountTypeAPIKey,
		Credentials:   map[string]any{"model_mapping": map[string]any{"claude-opus-4-8": "claude-opus-4-8"}},
		AccountGroups: []AccountGroup{{GroupID: groupB, AllowedModels: []string{"claude-opus-4-8", "claude-sonnet-4-6"}}},
	}
	require.True(t, mapped.IsModelSupportedInGroup(&groupB, "claude-opus-4-8"))
	require.False(t, mapped.IsModelSupportedInGroup(&groupB, "claude-sonnet-4-6"), "账号本身不支持的模型不会因为分组清单而放行")
}

func TestAccountsAllowedInGroupForModel(t *testing.T) {
	groupID := int64(5)
	accounts := []Account{
		{ID: 1, AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"gpt-5.5"}}}},
		{ID: 2, AccountGroups: []AccountGroup{{GroupID: groupID}}},
	}

	same := accountsAllowedInGroupForModel(accounts, &groupID, "gpt-5.5")
	require.Len(t, same, 2)
	require.Same(t, &accounts[0], &same[0], "没有账号被排除时原样返回，不拷贝")

	filtered := accountsAllowedInGroupForModel(accounts, &groupID, "gpt-5.4")
	require.Len(t, filtered, 1)
	require.Equal(t, int64(2), filtered[0].ID)

	require.Len(t, accountsAllowedInGroupForModel(accounts, nil, "gpt-5.4"), 2)
}

// groupAllowedModelsFixture 构造走负载感知路径的 GatewayService：分组 10 下有两个账号，
// 优先级更高的账号 1 在本分组只允许 claude-sonnet-4-6，账号 2 不限制。

func TestDuplicateAccountGroupsPreservesAllowedModels(t *testing.T) {
	original := &Account{AccountGroups: []AccountGroup{{GroupID: 2, Priority: 37, AllowedModels: []string{"gpt-5.5"}}}}
	groups, ids := duplicateAccountGroups(original)
	require.Equal(t, []int64{2}, ids)
	require.Equal(t, 37, groups[0].Priority)
	require.Equal(t, []string{"gpt-5.5"}, groups[0].AllowedModels)
	groups[0].AllowedModels[0] = "changed"
	require.Equal(t, []string{"gpt-5.5"}, original.AccountGroups[0].AllowedModels)
}
