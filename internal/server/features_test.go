package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deng23yu/stockagent/internal/eastmoney"
	"github.com/deng23yu/stockagent/internal/tencent"
	"github.com/deng23yu/stockagent/internal/track"
)

// TestHotSearches 验证热门搜索榜: 公开访问 (不受 admin token 影响)、排序与窗口、
// tracker 未启用时返回空数组。
func TestHotSearches(t *testing.T) {
	ts, tr := newVisitsTestServer(t, Options{AdminToken: "s3cret"})

	// 直接写库: 600737×2、600519×1、无 code×1、8 天前的老记录×1
	old := time.Now().AddDate(0, 0, -8)
	for _, v := range []track.Visit{
		{Time: time.Now(), IP: "1.1.1.1", Method: "GET", Path: "/api/v1/analyze", Code: "600737", Status: 200},
		{Time: time.Now(), IP: "1.1.1.2", Method: "GET", Path: "/api/v1/analyze", Code: "600737", Status: 200},
		{Time: time.Now(), IP: "1.1.1.3", Method: "GET", Path: "/api/v1/analyze", Code: "600519", Status: 200},
		{Time: time.Now(), IP: "1.1.1.4", Method: "GET", Path: "/", Status: 200},
		{Time: old, IP: "1.1.1.5", Method: "GET", Path: "/api/v1/analyze", Code: "000001", Status: 200},
	} {
		tr.Record(v)
	}
	waitVisit(t, tr, track.Filter{Code: "600519"}, nil)

	// 无 token 也应可访问 (公开接口)
	code, body := getURL(t, ts.URL+"/api/v1/hot-searches?days=7", nil)
	if code != 200 {
		t.Fatalf("hot-searches = %d, want 200 (公开)", code)
	}
	var out struct {
		Items []track.NameCount `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("items = %v, want 2 只 (老记录与无 code 被排除)", out.Items)
	}
	if out.Items[0].Name != "600737" || out.Items[0].Count != 2 {
		t.Errorf("榜首应为 600737×2: %+v", out.Items[0])
	}
	if out.Items[1].Name != "600519" || out.Items[1].Count != 1 {
		t.Errorf("第二应为 600519×1: %+v", out.Items[1])
	}

	// tracker 未启用 → 空数组
	ts2, _ := newVisitsTestServerNoTracker(t)
	code, body = getURL(t, ts2.URL+"/api/v1/hot-searches", nil)
	if code != 200 || !strings.Contains(string(body), `"items":[]`) {
		t.Errorf("未启用 tracker 应返回空数组: %d %s", code, body)
	}
}

func newVisitsTestServerNoTracker(t *testing.T) (*httptest.Server, interface{}) {
	t.Helper()
	srv, err := New(testConfig(), Options{CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, nil
}

// TestMarket 验证指数行情: 字段解析、走势图数据、60s 缓存 (上游只打一次)。
func TestMarket(t *testing.T) {
	var hits atomic.Int32
	emSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		secid := r.URL.Query().Get("secid")
		names := map[string]string{"1.000001": "上证指数", "0.399001": "深证成指", "0.399006": "创业板指"}
		name, ok := names[secid]
		if !ok {
			io.WriteString(w, `{"rc":0,"data":null}`)
			return
		}
		fmt.Fprintf(w, `{"rc":0,"data":{"f43":379628,"f57":"%s","f58":"%s","f170":85}}`, secid[2:], name)
	}))
	defer emSrv.Close()
	old := eastmoney.QuoteBaseURL
	eastmoney.QuoteBaseURL = emSrv.URL
	defer func() { eastmoney.QuoteBaseURL = old }()

	qqSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		symbol, _, _ := strings.Cut(r.URL.Query().Get("param"), ",")
		fmt.Fprintf(w, `{"code":0,"msg":"","data":{"%s":{"day":[`+
			`["2026-07-17","3865.32","3764.15","3869.21","3745.17","650"],`+
			`["2026-07-20","3791.66","3796.28","3831.66","3741.11","709"]]}}}`, symbol)
	}))
	defer qqSrv.Close()
	oldQQ := tencent.KlineURL
	tencent.KlineURL = qqSrv.URL
	defer func() { tencent.KlineURL = oldQQ }()

	srv, err := New(testConfig(), Options{CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var out struct {
		UpdatedAt string                 `json:"updated_at"`
		Indices   []eastmoney.IndexQuote `json:"indices"`
	}
	code, body := getURL(t, ts.URL+"/api/v1/market", nil)
	if code != 200 {
		t.Fatalf("market = %d: %s", code, body)
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Indices) != 3 {
		t.Fatalf("indices = %d, want 3", len(out.Indices))
	}
	if out.Indices[0].Name != "上证指数" || out.Indices[0].Price != 3796.28 || out.Indices[0].ChangePct != 0.85 {
		t.Errorf("上证指数解析异常: %+v", out.Indices[0])
	}
	if len(out.Indices[0].Closes) != 2 || out.Indices[0].Closes[1] != 3796.28 {
		t.Errorf("走势图 closes 异常: %+v", out.Indices[0].Closes)
	}

	// 60s 内第二次请求应命中缓存，上游调用数不变
	before := hits.Load()
	getURL(t, ts.URL+"/api/v1/market", nil)
	if got := hits.Load(); got != before {
		t.Errorf("缓存生效期上游被重复调用: %d -> %d", before, got)
	}
}

// TestCompare 验证多股对比: 参数校验、并行成功、共享缓存、每代码各记一条访客记录。
func TestCompare(t *testing.T) {
	llmSrv := setupMocks(t)
	cfg := testConfig()
	cfg.LLM.BaseURL = llmSrv.URL
	tr, err := track.Open(filepath.Join(t.TempDir(), "visits.db"), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(cfg, Options{CacheTTL: time.Minute, Tracker: tr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 参数校验
	for _, tc := range []struct{ url string }{
		{"/api/v1/compare"},                     // 缺 codes
		{"/api/v1/compare?codes=600519"},        // 少于 2 只
		{"/api/v1/compare?codes=600519,600519"}, // 去重后少于 2 只
		{"/api/v1/compare?codes=1,2,3,4,5"},     // 超过 4 只
		{"/api/v1/compare?codes=600519,abc123"}, // 非法代码
	} {
		if code, _ := getURL(t, ts.URL+tc.url, nil); code != 400 {
			t.Errorf("GET %s = %d, want 400", tc.url, code)
		}
	}

	// 正常对比: 2 只
	llmCalls.Store(0)
	code, body := getURL(t, ts.URL+"/api/v1/compare?codes=600519,000001", nil)
	if code != 200 {
		t.Fatalf("compare = %d: %s", code, body)
	}
	var out struct {
		Items []struct {
			Code   string          `json:"code"`
			OK     bool            `json:"ok"`
			Report json.RawMessage `json:"report"`
			Error  string          `json:"error"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || !out.Items[0].OK || !out.Items[1].OK {
		t.Fatalf("items 异常: %s", body[:200])
	}
	if out.Items[0].Code != "600519" || out.Items[1].Code != "000001" {
		t.Errorf("顺序应与输入一致: %s, %s", out.Items[0].Code, out.Items[1].Code)
	}
	firstCalls := llmCalls.Load()
	if firstCalls != 10 { // 2 只 × 5 次 LLM 调用
		t.Errorf("LLM 调用 = %d, want 10", firstCalls)
	}

	// 再次对比相同代码应全部命中缓存
	getURL(t, ts.URL+"/api/v1/compare?codes=600519,000001", nil)
	if got := llmCalls.Load(); got != firstCalls {
		t.Errorf("缓存命中后 LLM 调用 = %d, want %d (不变)", got, firstCalls)
	}

	// 对比请求展开为每代码一条访客记录 (热门榜依赖 code 列)
	waitVisit(t, tr, track.Filter{Path: "/api/v1/compare"}, func(v track.Visit) bool { return v.Code == "000001" })
	recs, err := tr.QueryVisits(100, track.Filter{Path: "/api/v1/compare"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, v := range recs {
		got[v.Code]++
	}
	if got["600519"] == 0 || got["000001"] == 0 {
		t.Errorf("对比的每个代码都应有记录: %v", got)
	}
}
