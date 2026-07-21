// Package tencent 封装腾讯行情免费接口 (ifzq.gtimg.cn)。
package tencent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// KlineURL 为日 K 接口地址，声明为变量以便测试中用 httptest 替换。
var KlineURL = "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get"

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) stockagent"

// Bar 是一根日 K (仅日期与收盘，快照/走势图够用)。
type Bar struct {
	Date  string // YYYY-MM-DD
	Close float64
}

// DailyBars 拉取最近 n 个交易日的日 K (升序)。symbol 形如 sh000001 / sz399001。
func DailyBars(ctx context.Context, symbol string, n int) ([]Bar, error) {
	if n <= 0 {
		n = 30
	}
	u := fmt.Sprintf("%s?param=%s,day,,,%d,qfq", KlineURL, symbol, n)
	body, err := get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data map[string]struct {
			Day    [][]any `json:"day"`
			QfqDay [][]any `json:"qfqday"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析腾讯 K 线响应: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("腾讯 K 线接口: %s (code %d)", resp.Msg, resp.Code)
	}
	d, ok := resp.Data[symbol]
	if !ok {
		return nil, fmt.Errorf("腾讯 K 线缺少 %s 数据", symbol)
	}
	bars := d.Day
	if len(bars) == 0 {
		bars = d.QfqDay
	}
	out := make([]Bar, 0, len(bars))
	for _, b := range bars {
		if len(b) < 3 {
			continue
		}
		date, _ := b[0].(string)
		cs, _ := b[2].(string)
		v, err := strconv.ParseFloat(cs, 64)
		if date == "" || err != nil || v <= 0 {
			continue
		}
		out = append(out, Bar{Date: date, Close: v})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("腾讯 K 线 %s 无有效数据", symbol)
	}
	return out, nil
}

// DailyCloses 拉取最近 n 个交易日的日收盘价 (升序)。
func DailyCloses(ctx context.Context, symbol string, n int) ([]float64, error) {
	bars, err := DailyBars(ctx, symbol, n)
	if err != nil {
		return nil, err
	}
	closes := make([]float64, 0, len(bars))
	for _, b := range bars {
		closes = append(closes, b.Close)
	}
	return closes, nil
}

// get 发起 GET 请求，网络错误重试一次。
func get(ctx context.Context, url string) ([]byte, error) {
	hc := &http.Client{Timeout: 10 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return body, nil
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil, lastErr
}
