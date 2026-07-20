package eastmoney

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecID(t *testing.T) {
	cases := map[string]string{
		"600519": "1.600519",
		"688981": "1.688981",
		"900901": "1.900901",
		"000001": "0.000001",
		"300750": "0.300750",
	}
	for code, want := range cases {
		got, err := SecID(code)
		if err != nil || got != want {
			t.Errorf("SecID(%q) = %q, %v; want %q", code, got, err, want)
		}
	}
	for _, bad := range []string{"12345", "6005190", "abcdef", "830799", ""} {
		if _, err := SecID(bad); err == nil {
			t.Errorf("SecID(%q) 应报错", bad)
		}
	}
}

const klineFixture = `{"rc":0,"rt":17,"data":{"code":"600519","market":1,"name":"贵州茅台","klines":[
"2026-07-01,1180.10,1193.01,1196.80,1166.33,42474",
"2026-07-02,-,-,-,-,-",
"2026-07-03,1205.24,1194.45,1210.14,1185.00,34268"]}}`

func TestKlines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("secid") != "1.600519" {
			t.Errorf("secid = %q, want 1.600519", r.URL.Query().Get("secid"))
		}
		w.Write([]byte(klineFixture))
	}))
	defer srv.Close()
	old := KlineBaseURL
	KlineBaseURL = srv.URL
	defer func() { KlineBaseURL = old }()

	d, err := New(nil).Klines(context.Background(), "1.600519", 10)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "贵州茅台" {
		t.Errorf("Name = %q", d.Name)
	}
	if len(d.Bars) != 2 {
		t.Fatalf("Bars = %d, want 2 (停牌行应跳过)", len(d.Bars))
	}
	b := d.Bars[0]
	if b.Date != "2026-07-01" || b.Open != 1180.10 || b.Close != 1193.01 ||
		b.High != 1196.80 || b.Low != 1166.33 || b.Volume != 42474 {
		t.Errorf("首根 K 线解析错误: %+v", b)
	}
}

const snapshotFixture = `{"rc":0,"rt":4,"data":{"f43":125300,"f44":126933,"f45":123898,
"f46":126901,"f57":"600519","f58":"贵州茅台","f60":125899,"f116":1.566352246053e+12,
"f117":1.566e+12,"f162":1437,"f167":664,"f168":47,"f170":-48}}`

func TestSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(snapshotFixture))
	}))
	defer srv.Close()
	old := QuoteBaseURL
	QuoteBaseURL = srv.URL
	defer func() { QuoteBaseURL = old }()

	s, err := New(nil).Snapshot(context.Background(), "1.600519")
	if err != nil {
		t.Fatal(err)
	}
	if s.Code != "600519" || s.Name != "贵州茅台" {
		t.Errorf("代码/名称错误: %+v", s)
	}
	if s.Price != 1253.00 || s.PrevClose != 1258.99 {
		t.Errorf("价格缩放错误: price=%v prev=%v", s.Price, s.PrevClose)
	}
	if s.PE != 14.37 || s.PB != 6.64 || s.ChangePct != -0.48 {
		t.Errorf("估值字段错误: PE=%v PB=%v chg=%v", s.PE, s.PB, s.ChangePct)
	}
	if s.TotalMarketCap != 1.566352246053e12 {
		t.Errorf("市值不应缩放: %v", s.TotalMarketCap)
	}
}

const annFixture = `{"data":{"list":[
{"art_code":"AN1","title":"贵州茅台:贵州茅台重大事项公告","notice_date":"2026-07-18 00:00:00"},
{"art_code":"AN2","title":"贵州茅台:2025年年度股东大会决议公告","notice_date":"2026-06-21 00:00:00"}]}}`

func TestAnnouncements(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("stock_list") != "600519" {
			t.Errorf("stock_list = %q", r.URL.Query().Get("stock_list"))
		}
		w.Write([]byte(annFixture))
	}))
	defer srv.Close()
	old := AnnBaseURL
	AnnBaseURL = srv.URL
	defer func() { AnnBaseURL = old }()

	anns, err := New(nil).Announcements(context.Background(), "600519", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 2 {
		t.Fatalf("公告数 = %d, want 2", len(anns))
	}
	if anns[0].Title != "贵州茅台:贵州茅台重大事项公告" || anns[0].Date != "2026-07-18" {
		t.Errorf("公告解析错误: %+v", anns[0])
	}
}

// TestGetFallbackHost 验证主域名失败时自动切换备用 (延时) 域名。
func TestGetFallbackHost(t *testing.T) {
	deadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("不支持 hijack")
		}
		conn, _, _ := hj.Hijack() // 直接断开，模拟连接被重置
		conn.Close()
	}))
	defer deadSrv.Close()
	aliveSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ok-from-delay")
	}))
	defer aliveSrv.Close()

	// 临时把备用表指向测试服务器
	old := fallbackHosts
	fallbackHosts = map[string]string{
		strings.TrimPrefix(deadSrv.URL, "http://"): strings.TrimPrefix(aliveSrv.URL, "http://"),
	}
	defer func() { fallbackHosts = old }()

	c := New(nil)
	body, err := c.get(context.Background(), deadSrv.URL+"/api/qt/stock/get")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok-from-delay" {
		t.Errorf("应来自备用域名, got %q", body)
	}
}

func TestSwapHost(t *testing.T) {
	got := swapHost("https://push2.eastmoney.com/api/qt/stock/get?secid=0.000001", fallbackHosts)
	want := "https://push2delay.eastmoney.com/api/qt/stock/get?secid=0.000001"
	if got != want {
		t.Errorf("swapHost = %q, want %q", got, want)
	}
	if got := swapHost("https://example.com/x", fallbackHosts); got != "" {
		t.Errorf("无匹配应为空串, got %q", got)
	}
}
