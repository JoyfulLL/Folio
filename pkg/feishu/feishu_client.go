package feishu

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const feishuResponseBodyLimit = 65536
const feishuErrorDetailLimit = 500

// FeishuClient represents a client which sends messages to a Feishu custom bot webhook.
// The webhook url and the signing secret are kept inside the struct and are never written
// to logs or error messages, because the webhook url itself contains credentials
type FeishuClient struct {
	webhookUrl    string
	signingSecret string
	keyword       string
	httpClient    *http.Client
}

// NewFeishuClient creates and returns a new feishu client with specified settings
func NewFeishuClient(webhookUrl string, signingSecret string, keyword string, httpClient *http.Client) *FeishuClient {
	return &FeishuClient{
		webhookUrl:    webhookUrl,
		signingSecret: signingSecret,
		keyword:       keyword,
		httpClient:    httpClient,
	}
}

// Sign returns the signature of a Feishu custom bot message. Feishu signs a message with
// HMAC-SHA256 using the string "timestamp\nsecret" as the key, an empty message body, and
// encodes the digest in base64
func Sign(timestamp string, signingSecret string) string {
	stringToSign := timestamp + "\n" + signingSecret
	mac := hmac.New(sha256.New, []byte(stringToSign))

	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// SendText sends a plain text message to the Feishu custom bot webhook
func (c *FeishuClient) SendText(text string) error {
	if c.keyword != "" {
		text = fmt.Sprintf("[%s]\n%s", c.keyword, text)
	}

	payload := map[string]any{
		"msg_type": "text",
		"content":  map[string]any{"text": text},
	}

	return c.send(payload)
}

// SendCard sends an interactive card message to the Feishu custom bot webhook. When the bot
// has custom-keyword protection enabled, the keyword element is inserted as the first card
// element so the card passes the verification
func (c *FeishuClient) SendCard(card map[string]any) error {
	cardPayload := make(map[string]any)

	for key, value := range card {
		cardPayload[key] = value
	}

	if c.keyword != "" {
		keywordElement := map[string]any{
			"tag":  "div",
			"text": map[string]any{"tag": "plain_text", "content": fmt.Sprintf("[%s]", c.keyword)},
		}

		existingElements := make([]any, 0)

		if existedElements, ok := cardPayload["elements"].([]any); ok {
			existingElements = append(existingElements, existedElements...)
		}

		cardPayload["elements"] = append([]any{keywordElement}, existingElements...)
	}

	payload := map[string]any{
		"msg_type": "interactive",
		"card":     cardPayload,
	}

	return c.send(payload)
}

// send signs the payload and posts it to the Feishu custom bot webhook. The webhook url is
// never included in any error message
func (c *FeishuClient) send(payload map[string]any) error {
	if c.webhookUrl == "" {
		return fmt.Errorf("feishu webhook url is not set")
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	payload["timestamp"] = timestamp
	payload["sign"] = Sign(timestamp, c.signingSecret)

	body, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("failed to marshal feishu payload: %s", err.Error())
	}

	request, err := http.NewRequest(http.MethodPost, c.webhookUrl, bytes.NewReader(body))

	if err != nil {
		return fmt.Errorf("failed to create feishu request: %s", err.Error())
	}

	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	response, err := c.httpClient.Do(request)

	if err != nil {
		return fmt.Errorf("feishu network request failed: %s", err.Error())
	}

	defer response.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(response.Body, feishuResponseBodyLimit))

	if err != nil {
		return fmt.Errorf("failed to read feishu response: %s", err.Error())
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("feishu returned http %d: %s", response.StatusCode, feishuTruncateString(string(rawBody), feishuErrorDetailLimit))
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(rawBody, &result); err != nil {
		return fmt.Errorf("feishu returned a non-json response: %s", feishuTruncateString(string(rawBody), 300))
	}

	if result.Code != 0 {
		return fmt.Errorf("feishu rejected the message (code=%d, msg=%s)", result.Code, result.Msg)
	}

	return nil
}

// feishuTruncateString returns at most maxRunes characters of the given string
func feishuTruncateString(value string, maxRunes int) string {
	runes := []rune(value)

	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}

	return value
}
