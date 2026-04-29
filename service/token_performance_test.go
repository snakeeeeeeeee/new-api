package service

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEstimateTokenByModelGolden(t *testing.T) {
	cases := []struct {
		name  string
		model string
		text  string
		want  int
	}{
		{
			name:  "english",
			model: "gpt-4o",
			text:  "Hello world, this is a token estimator test.",
			want:  12,
		},
		{
			name:  "chinese",
			model: "claude-sonnet-4-6",
			text:  "你好，世界。这个测试包含中文标点。",
			want:  19,
		},
		{
			name:  "url",
			model: "gemini-1.5-pro",
			text:  "https://example.com/a/b?x=1&y=test#frag",
			want:  26,
		},
		{
			name:  "math",
			model: "claude-3.5",
			text:  "E = mc², ∑(x_i) ≥ √4 and a/b=c",
			want:  34,
		},
		{
			name:  "emoji",
			model: "gpt-4o",
			text:  "emoji 😀🚀 mixed with text and 12345",
			want:  14,
		},
		{
			name:  "newline_tab",
			model: "gemini-2.0-flash",
			text:  "Line one\nLine two\tTabbed 42",
			want:  12,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, EstimateTokenByModel(tt.model, tt.text))
		})
	}
}

func TestCountTextTokenGolden(t *testing.T) {
	InitTokenEncoders()

	cases := []struct {
		name  string
		model string
		text  string
		want  int
	}{
		{
			name:  "openai_english",
			model: "gpt-4o",
			text:  "Hello world, this is a token estimator test.",
			want:  10,
		},
		{
			name:  "claude_chinese_estimator",
			model: "claude-sonnet-4-6",
			text:  "你好，世界。这个测试包含中文标点。",
			want:  19,
		},
		{
			name:  "gemini_url_estimator",
			model: "gemini-1.5-pro",
			text:  "https://example.com/a/b?x=1&y=test#frag",
			want:  26,
		},
		{
			name:  "openai_emoji",
			model: "gpt-4o",
			text:  "emoji 😀🚀 mixed with text and 12345",
			want:  11,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, CountTextToken(tt.text, tt.model))
		})
	}
}

func TestCountTextTokenContextCancellation(t *testing.T) {
	InitTokenEncoders()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	text := strings.Repeat("large canceled tokenization input ", 1024)
	require.Zero(t, CountTextTokenContext(ctx, text, "gpt-4o"))
	require.Zero(t, CountTextTokenContext(ctx, text, "claude-sonnet-4-6"))
}

func TestResponseText2UsageGolden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	usage := ResponseText2Usage(c, "E = mc², ∑(x_i) ≥ √4 and a/b=c", "claude-3.5", 7)

	require.Equal(t, 7, usage.PromptTokens)
	require.Equal(t, 34, usage.CompletionTokens)
	require.Equal(t, 41, usage.TotalTokens)
	require.True(t, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
}

var tokenBenchmarkResult int

func BenchmarkEstimateTokenByModelASCII(b *testing.B) {
	text := strings.Repeat("Hello world https://example.com/a/b?x=1&y=test#frag value=12345 ", 256)
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	for i := 0; i < b.N; i++ {
		tokenBenchmarkResult = EstimateTokenByModel("gemini-1.5-pro", text)
	}
}

func BenchmarkEstimateTokenByModelMathAndUnicode(b *testing.B) {
	text := strings.Repeat("你好，世界。E = mc², ∑(x_i) ≥ √4 and emoji 😀🚀\n", 256)
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	for i := 0; i < b.N; i++ {
		tokenBenchmarkResult = EstimateTokenByModel("claude-sonnet-4-6", text)
	}
}

func BenchmarkCountTextTokenOpenAI(b *testing.B) {
	InitTokenEncoders()
	text := strings.Repeat("Hello world, this is a tokenizer benchmark with numbers 12345. ", 256)
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenBenchmarkResult = CountTextToken(text, "gpt-4o")
	}
}

func BenchmarkResponseText2UsageFallback(b *testing.B) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	text := strings.Repeat("streaming response fallback text https://example.com/a?x=1&y=2\n", 256)

	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenBenchmarkResult = ResponseText2Usage(c, text, "claude-sonnet-4-6", 128).TotalTokens
	}
}
