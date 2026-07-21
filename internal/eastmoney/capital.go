package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// FFlowBaseURL 为资金流日 K 接口 (push2 系域名，失败自动回退 push2delay)。
var FFlowBaseURL = "https://push2.eastmoney.com/api/qt/stock/fflow/daykline/get"

// DataCenterURL 为东财数据中心接口 (两融/北向持股)。
var DataCenterURL = "https://datacenter-web.eastmoney.com/api/data/v1/get"

// MarginData 是个股两融数据 (最新披露日)。
type MarginData struct {
	Date    string  `json:"date"`
	RZYE    float64 `json:"rzye"`    // 融资余额 (元)
	RQYE    float64 `json:"rqye"`    // 融券余额 (元)
	RZRQYE  float64 `json:"rzrqye"`  // 两融余额 (元)
	RZMRE   float64 `json:"rzmre"`   // 当日融资买入额 (元)
	RZYEZB  float64 `json:"rzyezb"`  // 融资余额占流通市值比 (%)
	RZMRE5D float64 `json:"rzmre5d"` // 近 5 日融资买入额 (元)
}

// Margin 拉取个股最新两融数据。
func (c *Client) Margin(ctx context.Context, code string) (*MarginData, error) {
	u := fmt.Sprintf("%s?reportName=RPTA_WEB_RZRQ_GGMX&columns=ALL&pageNumber=1&pageSize=1"+
		"&sortColumns=DATE&sortTypes=-1&filter=(SCODE%%3D%%22%s%%22)", DataCenterURL, url.QueryEscape(code))
	row, err := c.dataCenterRow(ctx, u)
	if err != nil {
		return nil, err
	}
	m := &MarginData{
		Date:    dateOf(str(row["DATE"])),
		RZYE:    num(row["RZYE"], 1),
		RQYE:    num(row["RQYE"], 1),
		RZRQYE:  num(row["RZRQYE"], 1),
		RZMRE:   num(row["RZMRE"], 1),
		RZYEZB:  num(row["RZYEZB"], 1),
		RZMRE5D: num(row["RZMRE5D"], 1),
	}
	if m.Date == "" {
		return nil, fmt.Errorf("两融数据异常: 缺少日期 (%s)", code)
	}
	return m, nil
}

// FundFlowDay 是单日资金流向 (元，正为净流入)。
type FundFlowDay struct {
	Date       string  `json:"date"`
	Main       float64 `json:"main"`        // 主力净流入
	SuperLarge float64 `json:"super_large"` // 超大单净流入
	Large      float64 `json:"large"`       // 大单净流入
	Medium     float64 `json:"medium"`      // 中单净流入
	Small      float64 `json:"small"`       // 小单净流入
}

// FundFlow 拉取个股近 days 日资金流向 (升序)。secid 形如 1.600519。
func (c *Client) FundFlow(ctx context.Context, secid string, days int) ([]FundFlowDay, error) {
	if days <= 0 {
		days = 5
	}
	u := fmt.Sprintf("%s?secid=%s&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56&lmt=%d",
		FFlowBaseURL, url.QueryEscape(secid), days)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析资金流响应: %w", err)
	}
	out := make([]FundFlowDay, 0, len(resp.Data.Klines))
	for _, rec := range resp.Data.Klines {
		// "2026-07-21,主力,小单,中单,大单,超大单"
		f := strings.Split(rec, ",")
		if len(f) < 6 {
			continue
		}
		out = append(out, FundFlowDay{
			Date:       f[0],
			Main:       pf2(f[1]),
			Small:      pf2(f[2]),
			Medium:     pf2(f[3]),
			Large:      pf2(f[4]),
			SuperLarge: pf2(f[5]),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("未返回资金流数据 (%s)", secid)
	}
	return out, nil
}

// NorthboundHold 是沪深港通持股 (披露滞后，见 Date)。
type NorthboundHold struct {
	Date        string  `json:"date"`
	Shares      float64 `json:"shares"`       // 持股数量 (股)
	MarketCap   float64 `json:"market_cap"`   // 持股市值 (元)
	SharesRatio float64 `json:"shares_ratio"` // 占流通股比例 (%)
}

// Northbound 拉取个股沪深港通最新持股。
func (c *Client) Northbound(ctx context.Context, code string) (*NorthboundHold, error) {
	u := fmt.Sprintf("%s?reportName=RPT_MUTUAL_HOLDSTOCKNORTH_STA&columns=ALL&pageNumber=1&pageSize=1"+
		"&sortColumns=TRADE_DATE&sortTypes=-1&filter=(SECURITY_CODE%%3D%%22%s%%22)", DataCenterURL, url.QueryEscape(code))
	row, err := c.dataCenterRow(ctx, u)
	if err != nil {
		return nil, err
	}
	h := &NorthboundHold{
		Date:        dateOf(str(row["TRADE_DATE"])),
		Shares:      num(row["HOLD_SHARES"], 1),
		MarketCap:   num(row["HOLD_MARKET_CAP"], 1),
		SharesRatio: num(row["HOLD_SHARES_RATIO"], 1),
	}
	if h.Date == "" {
		return nil, fmt.Errorf("北向持股数据异常: 缺少日期 (%s)", code)
	}
	return h, nil
}

// dataCenterRow 请求 datacenter 接口并取首行 (result.data[0])。
func (c *Client) dataCenterRow(ctx context.Context, u string) (map[string]any, error) {
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Success bool `json:"success"`
		Result  struct {
			Data []map[string]any `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析数据中心响应: %w", err)
	}
	if len(resp.Result.Data) == 0 {
		return nil, fmt.Errorf("数据中心无数据")
	}
	return resp.Result.Data[0], nil
}

// dateOf 把 "2026-07-20 00:00:00" 截为 "2026-07-20"。
func dateOf(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func pf2(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
