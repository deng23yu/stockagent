package ths

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const klineFixture = `quotebridge_v6_line_hs_600519_01_last300({"rt":"0930-1130,1300-1500",` +
	`"data":"20260716,1252.00,1267.97,1245.05,1258.99,4761114,5987570900.00,0.381,,1800.00,2266182;` +
	`20260717,1269.01,1269.33,1238.98,1253.00,5841730,7322732700.00,0.467,,1700.00,2130100",` +
	`"marketType":"HS_stock_sh"})`

func TestKlinesByCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hs_600519/01/last300.js" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(klineFixture))
	}))
	defer srv.Close()
	old := KlineBaseURL
	KlineBaseURL = srv.URL
	defer func() { KlineBaseURL = old }()

	d, err := New(nil).KlinesByCode(context.Background(), "600519", 300)
	if err != nil {
		t.Fatal(err)
	}
	if d.Code != "600519" || len(d.Bars) != 2 {
		t.Fatalf("解析异常: code=%q bars=%d", d.Code, len(d.Bars))
	}
	b := d.Bars[1]
	if b.Date != "2026-07-17" || b.Open != 1269.01 || b.High != 1269.33 ||
		b.Low != 1238.98 || b.Close != 1253.00 {
		t.Errorf("末根 K 线解析错误: %+v", b)
	}
	if b.Volume != 58417.3 {
		t.Errorf("成交量应由股转换为手: %v", b.Volume)
	}
}

const quoteFixture = `quotebridge_v2_realhead_hs_600519_last({"items":{"10":"1253.00","6":"1258.99",` +
	`"7":"1269.01","8":"1269.33","9":"1238.98","199112":"-0.48","1968584":"0.467","2942":"14.374",` +
	`"3475914":"1566352200000.000","3541450":"1566352200000.000"},"name":"贵州茅台","5":"600519"})`

func TestSnapshotByCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hs_600519/last.js" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(quoteFixture))
	}))
	defer srv.Close()
	old := QuoteBaseURL
	QuoteBaseURL = srv.URL
	defer func() { QuoteBaseURL = old }()

	s, err := New(nil).SnapshotByCode(context.Background(), "600519")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "贵州茅台" || s.Code != "600519" {
		t.Errorf("代码/名称错误: %+v", s)
	}
	if s.Price != 1253.00 || s.PrevClose != 1258.99 || s.Open != 1269.01 ||
		s.High != 1269.33 || s.Low != 1238.98 {
		t.Errorf("价格字段映射错误: %+v", s)
	}
	if s.ChangePct != -0.48 || s.TurnoverPct != 0.467 || s.PE != 14.374 {
		t.Errorf("比率字段映射错误: %+v", s)
	}
	if s.TotalMarketCap != 1.5663522e12 {
		t.Errorf("市值字段错误: %v", s.TotalMarketCap)
	}
	if s.PB != 0 {
		t.Errorf("ths 不提供 PB, 应为 0: %v", s.PB)
	}
}

func TestValidateCode(t *testing.T) {
	for _, bad := range []string{"12345", "abcdef", "6005190"} {
		if _, err := New(nil).KlinesByCode(context.Background(), bad, 10); err == nil {
			t.Errorf("KlinesByCode(%q) 应报错", bad)
		}
	}
}

func TestStripJSONP(t *testing.T) {
	raw, err := stripJSONP([]byte(`cb({"a":1})`))
	if err != nil || string(raw) != `{"a":1}` {
		t.Errorf("stripJSONP = %q, %v", raw, err)
	}
	if _, err := stripJSONP([]byte(`not jsonp`)); err == nil {
		t.Error("非 JSONP 应报错")
	}
}
