package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDailyReportMaskWebhookUrl(t *testing.T) {
	// a normal feishu webhook url keeps the prefix and only the last 4 characters of the token
	masked := dailyReportMaskWebhookUrl("https://open.feishu.cn/open-apis/bot/v2/hook/abcd1234-ef56")
	assert.Equal(t, "https://open.feishu.cn/open-apis/bot/v2/hook/****ef56", masked)

	// a url without a slash is fully masked
	assert.Equal(t, "****", dailyReportMaskWebhookUrl("no-slash-url"))

	// a very short token is fully masked to avoid revealing almost the whole url
	assert.Equal(t, "****", dailyReportMaskWebhookUrl("https://x/ab"))
}
