//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPickGeminiCollectResultCarriesFinalUsageAndFinishReason(t *testing.T) {
	withParts := map[string]any{
		"candidates":    []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": "hi"}}}}},
		"usageMetadata": map[string]any{"promptTokenCount": 10, "candidatesTokenCount": 1},
	}
	final := map[string]any{
		"candidates":    []any{map[string]any{"finishReason": "MAX_TOKENS"}},
		"usageMetadata": map[string]any{"promptTokenCount": 10, "candidatesTokenCount": 42},
	}

	got := pickGeminiCollectResult(final, withParts)

	require.Equal(t, final["usageMetadata"], got["usageMetadata"])
	require.Equal(t, "MAX_TOKENS", extractGeminiFinishReason(got))
	require.Len(t, extractGeminiParts(got), 1)
	// 传入的分片不被修改。
	require.Equal(t, 1, withParts["usageMetadata"].(map[string]any)["candidatesTokenCount"])
	require.Empty(t, extractGeminiFinishReason(withParts))

	require.Equal(t, final, pickGeminiCollectResult(final, nil))
	require.Equal(t, withParts, pickGeminiCollectResult(nil, withParts))
	require.Empty(t, pickGeminiCollectResult(nil, nil))
}

func TestCollectClaudeStreamResponseUsesFinalChunkUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, nil)
	sse := strings.Join([]string{
		`data: {"response":{"candidates":[{"content":{"parts":[{"text":"hello "}]}}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":1}}}`,
		`data: {"response":{"candidates":[{"content":{"parts":[{"text":"world"}]}}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":2}}}`,
		`data: {"response":{"candidates":[{"finishReason":"MAX_TOKENS"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":57,"cachedContentTokenCount":40}}}`,
		"",
	}, "\n\n")
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(sse)), Header: http.Header{}}
	c, _ := newAntigravityCompatContext(http.MethodPost, "/v1/messages", nil)

	body, result, err := svc.collectClaudeStreamResponse(c, resp, time.Now(), "gemini-3.1-pro-high")

	require.NoError(t, err)
	require.Equal(t, 57, result.usage.OutputTokens)
	require.Equal(t, 40, result.usage.CacheReadInputTokens)
	require.Equal(t, "max_tokens", gjson.GetBytes(body, "stop_reason").String())
	require.Equal(t, "hello world", gjson.GetBytes(body, "content.#(type==\"text\").text").String())
}

func TestExtractGeminiUsageBillsToolUsePromptTokensAsInput(t *testing.T) {
	usage := extractGeminiUsage([]byte(`{"usageMetadata":{"promptTokenCount":100,"toolUsePromptTokenCount":30,"cachedContentTokenCount":40,"candidatesTokenCount":5,"thoughtsTokenCount":2}}`))

	require.NotNil(t, usage)
	require.Equal(t, 90, usage.InputTokens)
	require.Equal(t, 40, usage.CacheReadInputTokens)
	require.Equal(t, 7, usage.OutputTokens)
}
