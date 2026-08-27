package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBalanceHoldLedgerSourceSeparatesMobileVideoFromImage(t *testing.T) {
	image := &service.BatchImageBalanceHoldCommand{}
	video := &service.BatchImageBalanceHoldCommand{Kind: service.BalanceHoldKindMobileVideo}

	require.Equal(t, "image_balance_hold", balanceHoldLedgerSource(image, "hold"))
	require.Equal(t, "image_balance_capture", balanceHoldLedgerSource(image, "capture"))
	require.Equal(t, "image_balance_release", balanceHoldLedgerSource(image, "release"))
	require.Equal(t, "图片余额预留", balanceHoldLedgerDescription(image, "hold"))

	require.Equal(t, "video_balance_hold", balanceHoldLedgerSource(video, "hold"))
	require.Equal(t, "video_balance_capture", balanceHoldLedgerSource(video, "capture"))
	require.Equal(t, "video_balance_release", balanceHoldLedgerSource(video, "release"))
	require.Equal(t, "视频余额预留", balanceHoldLedgerDescription(video, "hold"))
	require.Equal(t, "视频费用结算", balanceHoldLedgerDescription(video, "capture"))
	require.Equal(t, "视频预留释放", balanceHoldLedgerDescription(video, "release"))
}
