// Package news 封装新浪财经 7×24 快讯接口。
package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FeedURL 为快讯接口地址，声明为变量以便测试中用 httptest 替换。
var FeedURL = "https://zhibo.sina.com.cn/api/zhibo/feed"

// Item 是一条快讯 (标题从 rich_text 的 【】 前缀解析，正文即全文，点击即读)。
type Item struct {
	ID      int64  `json:"id"`
	Time    string `json:"time"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Latest 拉取最新 n 条财经快讯。
func Latest(ctx context.Context, n int) ([]Item, error) {
	if n <= 0 || n > 50 {
		n = 30
	}
	u := fmt.Sprintf("%s?page=1&page_size=%d&zhibo_id=152&tag_id=0", FeedURL, n)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) stockagent")
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	hc := &http.Client{Timeout: 10 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("快讯接口 HTTP %d", resp.StatusCode)
	}

	var out struct {
		Result struct {
			Status struct {
				Code int `json:"code"`
			} `json:"status"`
			Data struct {
				Feed struct {
					List []struct {
						ID         int64  `json:"id"`
						RichText   string `json:"rich_text"`
						CreateTime string `json:"create_time"`
					} `json:"list"`
				} `json:"feed"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析快讯响应: %w", err)
	}
	if out.Result.Status.Code != 0 {
		return nil, fmt.Errorf("快讯接口错误码 %d", out.Result.Status.Code)
	}

	items := make([]Item, 0, len(out.Result.Data.Feed.List))
	for _, e := range out.Result.Data.Feed.List {
		text := strings.TrimSpace(e.RichText)
		if text == "" {
			continue
		}
		items = append(items, Item{ID: e.ID, Time: e.CreateTime, Title: titleOf(text), Content: text})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("快讯列表为空")
	}
	return items, nil
}

// titleOf 提取快讯标题: 【...】前缀，缺失时截前 30 字。
func titleOf(text string) string {
	if strings.HasPrefix(text, "【") {
		if i := strings.Index(text, "】"); i > 0 {
			return text[len("【"):i]
		}
	}
	r := []rune(text)
	if len(r) > 30 {
		return string(r[:30]) + "…"
	}
	return text
}
