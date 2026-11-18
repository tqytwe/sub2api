package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeUserGroupDeniedModels(t *testing.T) {
	models, err := NormalizeUserGroupDeniedModels(nil)
	require.NoError(t, err)
	require.Nil(t, models)

	models, err = NormalizeUserGroupDeniedModels([]string{" ", ""})
	require.NoError(t, err)
	require.Nil(t, models, "只有空白的清单等同于不限制")

	models, err = NormalizeUserGroupDeniedModels([]string{" gpt-6-luna ", "GPT-6-LUNA", "", "gpt-6-*"})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-6-luna", "gpt-6-*"}, models, "去掉空白与空项、按小写去重、保留填写顺序")

	_, err = NormalizeUserGroupDeniedModels([]string{"gpt-*-luna"})
	require.Error(t, err, "* 只能出现在末尾")

	_, err = NormalizeUserGroupDeniedModels([]string{strings.Repeat("m", maxUserGroupDeniedModelLength+1)})
	require.Error(t, err)

	tooMany := make([]string, 0, maxUserGroupDeniedModels+1)
	for i := 0; i <= maxUserGroupDeniedModels; i++ {
		tooMany = append(tooMany, fmt.Sprintf("model-%d", i))
	}
	_, err = NormalizeUserGroupDeniedModels(tooMany)
	require.Error(t, err)
}

func TestUserGroupDeniesModel(t *testing.T) {
	denied := []string{"gpt-6-luna", "claude-opus-*", "gemini-2.5-pro"}
	cases := []struct {
		model string
		want  bool
	}{
		{"gpt-6-luna", true},
		{"GPT-6-Luna", true},
		{"gpt-6-sol", false},
		{"claude-opus-4-8", true},
		{"claude-sonnet-4-6", false},
		{"models/gemini-2.5-pro", true},
		{"", false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, UserGroupDeniesModel(denied, tc.model), tc.model)
	}
	require.False(t, UserGroupDeniesModel(nil, "gpt-6-luna"), "没有禁用清单时不拦截")

	require.Equal(t, "gpt-6-luna", FirstUserGroupDeniedModel(denied, []string{"gpt-6-sol", " gpt-6-luna "}))
	require.Equal(t, "", FirstUserGroupDeniedModel(denied, []string{"gpt-6-sol"}))

	require.Equal(t, []string{"gpt-6-sol"}, FilterUserGroupDeniedModelIDs([]string{"gpt-6-luna", "gpt-6-sol"}, denied))
	ids := []string{"gpt-6-luna"}
	require.Equal(t, ids, FilterUserGroupDeniedModelIDs(ids, nil))
}
