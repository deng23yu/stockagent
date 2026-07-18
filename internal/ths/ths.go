// Package ths 封装同花顺免费行情接口 (d.10jqka.com.cn)。
//
// 接口为 JSONP 格式、字段数字编码，本包负责去包装与字段映射。
// 字段含义基于真实响应与 K 线交叉验证 (见 snapshot.go 注释与 live_test.go)。
package ths

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 接口 base URL，声明为变量以便测试中用 httptest 替换。
var (
	KlineBaseURL = "https://d.10jqka.com.cn/v6/line"     // {base}/hs_600519/01/last300.js
	QuoteBaseURL = "https://d.10jqka.com.cn/v2/realhead" // {base}/hs_600519/last.js
)

const (
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) stockagent"
	referer   = "https://stockpage.10jqka.com.cn/"
)

// Client 是同花顺数据客户端，实现 eastmoney.Source。
type Client struct {
	hc *http.Client
}

// New 创建 Client；hc 为 nil 时使用带 15s 超时的默认客户端。
func New(hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{hc: hc}
}

// get 发起 GET 请求并返回响应体，对网络错误与 5xx 重试一次。
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Referer", referer)

		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return body, nil
		}
		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		if resp.StatusCode < 500 {
			break
		}
	}
	return nil, lastErr
}

// stripJSONP 提取 callback({...}) 中的 JSON 对象。
func stripJSONP(body []byte) ([]byte, error) {
	s := string(body)
	i := strings.Index(s, "(")
	j := strings.LastIndex(s, ")")
	if i < 0 || j <= i {
		return nil, fmt.Errorf("非预期 JSONP 格式: %.80s…", s)
	}
	return []byte(s[i+1 : j]), nil
}

// validateCode 校验 6 位数字代码 (ths 的 hs_ 前缀沪深通用)。
func validateCode(code string) error {
	if len(code) != 6 {
		return fmt.Errorf("股票代码应为 6 位数字，收到 %q", code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return fmt.Errorf("股票代码应为 6 位数字，收到 %q", code)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
