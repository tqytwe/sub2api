package service

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestLeaderboardEmailMasksBothPartsWithoutBreakingUnicode(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"alice@gmail.com", "al***@g***.com"},
		{" 张三丰@example.com ", "张三***@e***.com"},
		{"a@x.io", "a***@x***.io"},
		{"ab@mail.example.com", "a***@m***.com"},
		{"", ""}, {"no-address", ""}, {"a@@example.com", ""},
		{"a@localhost", ""}, {"a@.com", ""}, {"a b@example.com", ""},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got := MaskLeaderboardEmail(tt.input)
			require.Equal(t, tt.want, got)
			require.True(t, utf8.ValidString(got))
		})
	}
}
