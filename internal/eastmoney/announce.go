package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Announcement 是一条公告。
type Announcement struct {
	Title string `json:"title"`
	Date  string `json:"date"`
}

// Announcements 拉取最近公告，按时间倒序，最多 limit 条。
func (c *Client) Announcements(ctx context.Context, code string, limit int) ([]Announcement, error) {
	u := fmt.Sprintf("%s?sr=-1&page_size=%d&page_index=1&ann_type=A&client_source=web&stock_list=%s",
		AnnBaseURL, limit, url.QueryEscape(code))

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data *struct {
			List []struct {
				Title      string `json:"title"`
				NoticeDate string `json:"notice_date"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析公告响应: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("未返回公告数据")
	}
	var out []Announcement
	for _, item := range resp.Data.List {
		date := item.NoticeDate
		if f := strings.Fields(date); len(f) > 0 {
			date = f[0]
		}
		out = append(out, Announcement{Title: item.Title, Date: date})
	}
	return out, nil
}
