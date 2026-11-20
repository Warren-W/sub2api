//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const antigravityQuotaExhaustedBody = `{
	"error": {
		"code": 429,
		"message": "You have exhausted your capacity on this model. Your quota will reset after 4h.",
		"status": "RESOURCE_EXHAUSTED",
		"details": [
			{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "QUOTA_EXHAUSTED",
			 "metadata": {"model": "gemini-3.1-pro-high", "quotaResetDelay": "4h0m0s"}}
		]
	}
}`

func TestParseAntigravitySmartRetryInfo_QuotaExhausted(t *testing.T) {
	info := parseAntigravitySmartRetryInfo([]byte(antigravityQuotaExhaustedBody))
	require.NotNil(t, info)
	require.True(t, info.IsQuotaExhausted)
	require.False(t, info.IsModelCapacityExhausted)
	require.Equal(t, "gemini-3.1-pro-high", info.ModelName)
	require.Equal(t, 4*time.Hour, info.RetryDelay, "无 RetryInfo 时取 metadata.quotaResetDelay")

	withRetryInfo := []byte(`{"error":{"status":"RESOURCE_EXHAUSTED","details":[
		{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"QUOTA_EXHAUSTED","metadata":{"model":"gemini-3.1-pro-high","quotaResetDelay":"4h0m0s"}},
		{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"120s"}]}}`)
	info = parseAntigravitySmartRetryInfo(withRetryInfo)
	require.NotNil(t, info)
	require.Equal(t, 120*time.Second, info.RetryDelay, "RetryInfo 优先")

	// 没有模型名的响应维持原行为：不解析。
	noModel := []byte(`{"error":{"status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"QUOTA_EXHAUSTED","metadata":{"quotaResetDelay":"2h0m0s"}}]}}`)
	require.Nil(t, parseAntigravitySmartRetryInfo(noModel))
}

func TestShouldTriggerAntigravitySmartRetry_QuotaExhaustedSwitchesAccount(t *testing.T) {
	account := &Account{Type: AccountTypeOAuth, Platform: PlatformAntigravity}
	shouldRetry, shouldRateLimit, wait, model, capacity := shouldTriggerAntigravitySmartRetry(account, []byte(antigravityQuotaExhaustedBody))
	require.False(t, shouldRetry)
	require.True(t, shouldRateLimit)
	require.Equal(t, 4*time.Hour, wait)
	require.Equal(t, "gemini-3.1-pro-high", model)
	require.False(t, capacity)
}

func TestHandleSmartRetry_QuotaExhausted_ReturnsSwitchErrorWithoutRetry(t *testing.T) {
	repo := &stubAntigravityAccountRepo{}
	account := &Account{ID: 7, Name: "acc-7", Type: AccountTypeOAuth, Platform: PlatformAntigravity}
	respBody := []byte(antigravityQuotaExhaustedBody)
	resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(respBody))}
	params := antigravityRetryLoopParams{
		ctx:             context.Background(),
		prefix:          "[test]",
		account:         account,
		accessToken:     "token",
		action:          "generateContent",
		body:            []byte(`{"input":"test"}`),
		accountRepo:     repo,
		isStickySession: true,
		handleError: func(context.Context, string, *Account, int, http.Header, []byte, string, int64, string, bool) *handleModelRateLimitResult {
			return nil
		},
	}

	result := (&AntigravityGatewayService{}).handleSmartRetry(params, resp, respBody, "https://ag-1.test", 0, []string{"https://ag-1.test"})

	require.NotNil(t, result)
	require.Equal(t, smartRetryActionBreakWithResp, result.action)
	require.NotNil(t, result.switchError)
	require.Equal(t, account.ID, result.switchError.OriginalAccountID)
	require.Equal(t, "gemini-3.1-pro-high", result.switchError.RateLimitedModel)
	// Gemini 各模型共用额度：模型本身和 antigravity:gemini 家族一起冷却到额度重置。
	require.Len(t, repo.modelRateLimitCalls, 2)
	require.Equal(t, "gemini-3.1-pro-high", repo.modelRateLimitCalls[0].modelKey)
	require.Equal(t, "antigravity:gemini", repo.modelRateLimitCalls[1].modelKey)
	for _, call := range repo.modelRateLimitCalls {
		require.WithinDuration(t, time.Now().Add(4*time.Hour), call.resetAt, time.Minute)
	}
}

func TestShouldMarkCreditsExhausted_QuotaExhaustedKeepsKeywordJudgement(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests}
	require.False(t, shouldMarkCreditsExhausted(resp, []byte(antigravityQuotaExhaustedBody), nil))

	withCreditsHint := []byte(`{"error":{"status":"RESOURCE_EXHAUSTED","message":"Insufficient credits","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"QUOTA_EXHAUSTED","metadata":{"model":"gemini-3.1-pro-high"}}]}}`)
	require.True(t, shouldMarkCreditsExhausted(resp, withCreditsHint, nil))

	require.Equal(t, antigravity429QuotaExhausted, classifyAntigravity429([]byte(antigravityQuotaExhaustedBody)))
}
