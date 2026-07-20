package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// IndexQuote 是指数实时行情。价格为点位，ChangePct 为百分数。
type IndexQuote struct {
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	ChangePct float64   `json:"change_pct"`
	Closes    []float64 `json:"closes,omitempty"` // 近 30 日收盘价 (走势图数据，可选)
}

// DefaultIndexIDs 为常用指数 secid (沪市 1.*, 深市 0.*): 上证指数、深证成指、创业板指。
var DefaultIndexIDs = []string{"1.000001", "0.399001", "0.399006"}

// IndexQuotes 拉取指数行情 (逐个调用 quote 接口，复用 delay 域名回退)。
// 单个指数失败则跳过，全部失败才返回错误。
func (c *Client) IndexQuotes(ctx context.Context, secids []string) ([]IndexQuote, error) {
	var out []IndexQuote
	var lastErr error
	for _, id := range secids {
		q, err := c.indexQuote(ctx, id)
		if err != nil {
			lastErr = err
			continue
		}
		out = append(out, q)
	}
	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}

func (c *Client) indexQuote(ctx context.Context, secid string) (IndexQuote, error) {
	u := fmt.Sprintf("%s?secid=%s&fields=f43,f57,f58,f170", QuoteBaseURL, url.QueryEscape(secid))
	body, err := c.get(ctx, u)
	if err != nil {
		return IndexQuote{}, err
	}
	var resp struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return IndexQuote{}, fmt.Errorf("解析指数响应: %w", err)
	}
	if resp.Data == nil {
		return IndexQuote{}, fmt.Errorf("未返回指数数据: %s", secid)
	}
	q := IndexQuote{
		Code:      str(resp.Data["f57"]),
		Name:      str(resp.Data["f58"]),
		Price:     num(resp.Data["f43"], 100),
		ChangePct: num(resp.Data["f170"], 100),
	}
	if q.Name == "" {
		return IndexQuote{}, fmt.Errorf("指数数据异常: 缺少名称 (%s)", secid)
	}
	return q, nil
}
