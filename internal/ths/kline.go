package ths

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/deng23yu/stockagent/internal/eastmoney"
)

// KlinesByCode 拉取日 K 线 (前复权)，最多 limit 根，按日期升序。
// 记录格式: date,open,high,low,close,volume(股),amount,turnover,...
func (c *Client) KlinesByCode(ctx context.Context, code string, limit int) (*eastmoney.KlineData, error) {
	if err := validateCode(code); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/hs_%s/01/last%d.js", KlineBaseURL, code, limit)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	raw, err := stripJSONP(body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("解析 K 线响应: %w", err)
	}
	if resp.Data == "" {
		return nil, fmt.Errorf("未返回 K 线数据，请检查股票代码是否正确")
	}

	d := &eastmoney.KlineData{Code: code}
	for _, rec := range strings.Split(resp.Data, ";") {
		if k, ok := parseRecord(rec); ok {
			d.Bars = append(d.Bars, k)
		}
	}
	if len(d.Bars) == 0 {
		return nil, fmt.Errorf("K 线数据全部无效")
	}
	return d, nil
}

// parseRecord 解析 "20260717,1269.01,1269.33,1238.98,1253.00,5841730,7322732700.00,0.467,,,"
// (date,open,high,low,close,volume,amount,...)。成交量由股转换为手。
func parseRecord(rec string) (eastmoney.Kline, bool) {
	var k eastmoney.Kline
	f := strings.Split(rec, ",")
	if len(f) < 6 || len(f[0]) != 8 {
		return k, false
	}
	open, high, low, close := pf(f[1]), pf(f[2]), pf(f[3]), pf(f[4])
	if close <= 0 {
		return k, false
	}
	k = eastmoney.Kline{
		Date:   f[0][:4] + "-" + f[0][4:6] + "-" + f[0][6:8],
		Open:   open,
		High:   high,
		Low:    low,
		Close:  close,
		Volume: pf(f[5]) / 100,
	}
	return k, true
}

func pf(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
