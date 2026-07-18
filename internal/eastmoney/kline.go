package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// SecID 将 6 位 A 股代码转换为东方财富 secid (market.code)。
// 沪市 (6/9 开头) → 1.code，深市 (0/2/3 开头) → 0.code。
// 北交所等市场暂不支持 (roadmap)。
func SecID(code string) (string, error) {
	if len(code) != 6 {
		return "", fmt.Errorf("股票代码应为 6 位数字，收到 %q", code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("股票代码应为 6 位数字，收到 %q", code)
		}
	}
	switch code[0] {
	case '6', '9':
		return "1." + code, nil
	case '0', '2', '3':
		return "0." + code, nil
	default:
		return "", fmt.Errorf("暂不支持代码 %q 所属的市场 (北交所等在 roadmap 中)", code)
	}
}

// Kline 是一根日 K 线。
type Kline struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Volume float64 `json:"volume"` // 单位: 手
}

// KlineData 是个股的 K 线序列及元信息。
type KlineData struct {
	Code string  `json:"code"`
	Name string  `json:"name"`
	Bars []Kline `json:"bars"`
}

type klineResponse struct {
	Data *struct {
		Code   string   `json:"code"`
		Name   string   `json:"name"`
		Klines []string `json:"klines"`
	} `json:"data"`
}

// Klines 拉取前复权日 K 线，最多 limit 根，按日期升序。
func (c *Client) Klines(ctx context.Context, secid string, limit int) (*KlineData, error) {
	u := fmt.Sprintf("%s?secid=%s&fields1=f1,f2,f3&fields2=f51,f52,f53,f54,f55,f56&klt=101&fqt=1&beg=0&end=20500101&lmt=%d",
		KlineBaseURL, url.QueryEscape(secid), limit)

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp klineResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 K 线响应: %w", err)
	}
	if resp.Data == nil || len(resp.Data.Klines) == 0 {
		return nil, fmt.Errorf("未返回 K 线数据，请检查股票代码是否正确")
	}

	d := &KlineData{Code: resp.Data.Code, Name: resp.Data.Name}
	for _, line := range resp.Data.Klines {
		if k, ok := parseKlineLine(line); ok {
			d.Bars = append(d.Bars, k)
		}
	}
	if len(d.Bars) == 0 {
		return nil, fmt.Errorf("K 线数据全部无效")
	}
	return d, nil
}

// parseKlineLine 解析 "2026-07-01,1180.10,1193.01,1196.80,1166.33,42474"
// (日期,开,收,高,低,成交量)。停牌等情况下字段为 "-"，返回 ok=false。
func parseKlineLine(line string) (k Kline, ok bool) {
	parts := strings.Split(line, ",")
	if len(parts) < 6 {
		return k, false
	}
	vals := make([]float64, 5)
	for i, p := range parts[1:6] {
		if p == "-" || p == "" {
			return k, false
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return k, false
		}
		vals[i] = v
	}
	return Kline{
		Date:   parts[0],
		Open:   vals[0],
		Close:  vals[1],
		High:   vals[2],
		Low:    vals[3],
		Volume: vals[4],
	}, true
}
