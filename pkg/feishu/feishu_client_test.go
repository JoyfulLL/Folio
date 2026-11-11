package feishu

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSign(t *testing.T) {
	// the expected values are precomputed with
	// hmac.new(key=f"{timestamp}\n{secret}", msg=b"", digestmod=sha256).digest(), base64-encoded
	assert.Equal(t, "mbm4Y4oluIPQ00qlBIhX8vAZ0EKv3nw0LuTb91jPL84=", Sign("1700000000", "test-secret"))
	assert.Equal(t, "U7aU5cUOz1gwzGq7J+Rsv10XhLMbM/PgEWCv8h8XI0M=", Sign("1731200000", "abc"))
	assert.Equal(t, Sign("1700000000", "test-secret"), Sign("1700000000", "test-secret"))
}

func TestSendText(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()

	client := NewFeishuClient(server.URL, "test-secret", "记账", server.Client())
	err := client.SendText("日报测试")

	assert.Nil(t, err)
	assert.Equal(t, "text", received["msg_type"])
	assert.NotEmpty(t, received["timestamp"])
	assert.NotEmpty(t, received["sign"])

	content, ok := received["content"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "[记账]\n日报测试", content["text"])
}

func TestSendCardWithKeyword(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()

	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title": map[string]any{"tag": "plain_text", "content": "记账日报"},
		},
		"elements": []any{
			map[string]any{"tag": "hr"},
		},
	}

	client := NewFeishuClient(server.URL, "test-secret", "记账", server.Client())
	err := client.SendCard(card)

	assert.Nil(t, err)
	assert.Equal(t, "interactive", received["msg_type"])

	cardPayload, ok := received["card"].(map[string]any)
	assert.True(t, ok)

	elements, ok := cardPayload["elements"].([]any)
	assert.True(t, ok)
	assert.Len(t, elements, 2)

	keywordElement, ok := elements[0].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "div", keywordElement["tag"])

	// the original elements are preserved after the keyword element
	hrElement, ok := elements[1].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "hr", hrElement["tag"])

	header, ok := cardPayload["header"].(map[string]any)
	assert.True(t, ok)
	title, ok := header["title"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "记账日报", title["content"])
}

func TestSendCardWithoutKeywordKeepsOriginalOrder(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()

	card := map[string]any{
		"elements": []any{map[string]any{"tag": "hr"}},
	}

	client := NewFeishuClient(server.URL, "test-secret", "", server.Client())
	err := client.SendCard(card)

	assert.Nil(t, err)

	cardPayload, ok := received["card"].(map[string]any)
	assert.True(t, ok)

	elements, ok := cardPayload["elements"].([]any)
	assert.True(t, ok)
	assert.Len(t, elements, 1)
}

func TestSendRejectedByFeishu(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":19021,"msg":"sign match fail"}`))
	}))
	defer server.Close()

	client := NewFeishuClient(server.URL, "wrong-secret", "", server.Client())
	err := client.SendText("hello")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "code=19021")
	assert.Contains(t, err.Error(), "sign match fail")
}

func TestSendHttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer server.Close()

	client := NewFeishuClient(server.URL, "test-secret", "", server.Client())
	err := client.SendText("hello")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "http 403")
	assert.Contains(t, err.Error(), "forbidden")
	// the webhook url itself must never appear in error messages
	assert.NotContains(t, err.Error(), server.URL)
}

func TestSendWithEmptyWebhookUrl(t *testing.T) {
	client := NewFeishuClient("", "test-secret", "", &http.Client{})
	err := client.SendText("hello")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "webhook url is not set")
}

func TestSendNonJsonResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html>blocked</html>`))
	}))
	defer server.Close()

	client := NewFeishuClient(server.URL, "test-secret", "", server.Client())
	err := client.SendText("hello")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "non-json response")
}
