package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Snapshot 是个股实时行情与估值快照。金额为元，比率为百分数。
type Snapshot struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	Open           float64 `json:"open"`
	High           float64 `json:"high"`
	Low            float64 `json:"low"`
	PrevClose      float64 `json:"prev_close"`
	ChangePct      float64 `json:"change_pct"`
	TurnoverPct    float64 `json:"turnover_pct"`
	PE             float64 `json:"pe"`               // 动态市盈率
	PB             float64 `json:"pb"`               // 市净率
	TotalMarketCap float64 `json:"total_market_cap"` // 总市值 (元)
	FloatMarketCap float64 `json:"float_market_cap"` // 流通市值 (元)
}

// Snapshot 拉取实时行情快照。A 股价格类字段按 ×100 缩放编码，停牌字段为 "-"。
func (c *Client) Snapshot(ctx context.Context, secid string) (*Snapshot, error) {
	u := fmt.Sprintf("%s?secid=%s&fields=f43,f44,f45,f46,f57,f58,f60,f116,f117,f162,f167,f168,f170",
		QuoteBaseURL, url.QueryEscape(secid))

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析快照响应: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("未返回快照数据，请检查股票代码是否正确")
	}
	d := resp.Data
	s := &Snapshot{
		Code:           str(d["f57"]),
		Name:           str(d["f58"]),
		Price:          num(d["f43"], 100),
		High:           num(d["f44"], 100),
		Low:            num(d["f45"], 100),
		Open:           num(d["f46"], 100),
		PrevClose:      num(d["f60"], 100),
		ChangePct:      num(d["f170"], 100),
		TurnoverPct:    num(d["f168"], 100),
		PE:             num(d["f162"], 100),
		PB:             num(d["f167"], 100),
		TotalMarketCap: num(d["f116"], 1),
		FloatMarketCap: num(d["f117"], 1),
	}
	if s.Name == "" {
		return nil, fmt.Errorf("快照数据异常: 缺少股票名称")
	}
	return s, nil
}

// num 解析缩放数字字段；停牌等场景的 "-" 字符串返回 0。
func num(v any, scale float64) float64 {
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return f / scale
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
