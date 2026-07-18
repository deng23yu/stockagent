// Package llm 是一个极简的 OpenAI 兼容 Chat Completions 客户端。
//
// 兼容 DeepSeek / 通义 / Kimi / OpenAI / Ollama 等一切
// 暴露 /chat/completions 端点的服务。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 是 LLM 客户端。
type Client struct {
	baseURL string
	apiKey  string
	model   string
	hc      *http.Client
}

// New 创建 Client。baseURL 形如 "https://api.deepseek.com"，可带 /v1。
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		hc:      &http.Client{Timeout: 90 * time.Second},
	}
}

// Model 返回当前使用的模型名。
func (c *Client) Model() string { return c.model }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 发起一轮 system+user 对话并返回助手文本。
// jsonMode 时请求 response_format=json_object 并在失败时重试一次。
func (c *Client) Chat(ctx context.Context, system, user string, jsonMode bool) (string, error) {
	reqBody := chatRequest{
		Model:       c.model,
		Temperature: 0.3,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	if jsonMode {
		reqBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.baseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return parseContent(body)
		}
		lastErr = fmt.Errorf("LLM HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			break // 4xx (除 429) 重试无意义
		}
	}
	return "", lastErr
}

func parseContent(body []byte) (string, error) {
	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("解析 LLM 响应: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("LLM 错误: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 || cr.Choices[0].Message.Content == "" {
		return "", errors.New("LLM 返回为空")
	}
	return cr.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
