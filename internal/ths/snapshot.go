package ths

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deng23yu/stockagent/internal/eastmoney"
)

// SnapshotByCode 拉取实时快照。
//
// 字段映射 (基于真实响应与 K 线数据交叉验证):
//
//	10=最新价  6=昨收  7=今开  8=最高  9=最低
//	199112=涨跌幅%  1968584=换手率%  2942=动态PE
//	3475914=总市值(元)  3541450=流通市值(元)
//
// 接口不提供 PB，返回 0 (调用方按数据缺失处理)。
func (c *Client) SnapshotByCode(ctx context.Context, code string) (*eastmoney.Snapshot, error) {
	if err := validateCode(code); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/hs_%s/last.js", QuoteBaseURL, code)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	raw, err := stripJSONP(body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items map[string]any `json:"items"` // 值以字符串为主, 混有少量数值 (如 "stop":0)
		Name  string         `json:"name"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("解析快照响应: %w", err)
	}
	if resp.Items == nil {
		return nil, fmt.Errorf("未返回快照数据，请检查股票代码是否正确")
	}
	it := resp.Items
	// name 字段位置不稳定: 有时在顶层, 有时在 items 内
	name := resp.Name
	if name == "" {
		name, _ = it["name"].(string)
	}
	s := &eastmoney.Snapshot{
		Code:           code,
		Name:           name,
		Price:          pfAny(it["10"]),
		Open:           pfAny(it["7"]),
		High:           pfAny(it["8"]),
		Low:            pfAny(it["9"]),
		PrevClose:      pfAny(it["6"]),
		ChangePct:      pfAny(it["199112"]),
		TurnoverPct:    pfAny(it["1968584"]),
		PE:             pfAny(it["2942"]),
		PB:             0, // ths 快照接口不提供 PB
		TotalMarketCap: pfAny(it["3475914"]),
		FloatMarketCap: pfAny(it["3541450"]),
	}
	if s.Name == "" || s.Price <= 0 {
		return nil, fmt.Errorf("快照数据异常，请检查股票代码是否正确")
	}
	return s, nil
}

// pfAny 解析字符串或数值型字段。
func pfAny(v any) float64 {
	switch t := v.(type) {
	case string:
		return pf(t)
	case float64:
		return t
	}
	return 0
}
