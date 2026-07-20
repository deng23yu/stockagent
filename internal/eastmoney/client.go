// Package eastmoney 封装东方财富免费行情数据接口。
//
// 接口字段采用缩放整数编码 (如 f43=125300 表示价格 1253.00)，
// 本包负责解码与缩放还原。
package eastmoney

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 各接口的 base URL，声明为变量以便测试中用 httptest 替换。
var (
	KlineBaseURL = "https://push2his.eastmoney.com/api/qt/stock/kline/get"
	QuoteBaseURL = "https://push2.eastmoney.com/api/qt/stock/get"
	AnnBaseURL   = "https://np-anotice-stock.eastmoney.com/api/security/ann"
)

// fallbackHosts 为主域名不可用时的备用域名 (延时行情，数据略有延迟但接口一致)。
// 东财主域名偶尔会对机房 IP 做连接重置，实测延时域名不受牵连。
var fallbackHosts = map[string]string{
	"push2his.eastmoney.com": "push2delay.eastmoney.com",
	"push2.eastmoney.com":    "push2delay.eastmoney.com",
}

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) stockagent"

// Client 是东方财富数据客户端。
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

// get 发起 GET 请求并返回响应体，对网络错误与 5xx 重试一次；
// 全部失败且域名存在备用时，换备用 (延时) 域名再试一次。
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	body, err := c.getAttempts(ctx, url, 2)
	if err != nil {
		if alt := swapHost(url, fallbackHosts); alt != "" {
			return c.getAttempts(ctx, alt, 1)
		}
	}
	return body, err
}

// swapHost 按表替换 URL 域名，无匹配返回空串。
func swapHost(rawurl string, table map[string]string) string {
	for old, newHost := range table {
		if strings.Contains(rawurl, "://"+old+"/") {
			return strings.Replace(rawurl, "://"+old+"/", "://"+newHost+"/", 1)
		}
	}
	return ""
}

// getAttempts 发起 GET 请求，最多 attempts 次 (网络错误与 5xx 重试)。
func (c *Client) getAttempts(ctx context.Context, url string, attempts int) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
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
		req.Header.Set("Referer", "https://quote.eastmoney.com/")

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

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
