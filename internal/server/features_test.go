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
	"github.com/deng23yu/stockagent/internal/news"
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

// TestActivityEndpoint 验证实时动态接口: 公开访问、compare 合并、内网排除、不含 IP。
func TestActivityEndpoint(t *testing.T) {
	ts, tr := newVisitsTestServer(t, Options{AdminToken: "s3cret"})

	tr.Record(track.Visit{Time: time.Now(), IP: "203.0.113.9", Method: "GET", Path: "/api/v1/analyze", Code: "600737", Status: 200})
	for _, c := range []string{"600519", "000001"} {
		tr.Record(track.Visit{Time: time.Now(), IP: "203.0.113.10", Method: "GET", Path: "/api/v1/compare",
			Query: "codes=600519,000001", Code: c, Status: 200})
	}
	waitVisit(t, tr, track.Filter{Code: "600737"}, nil)

	// 无 token 也应可访问 (公开接口)
	code, body := getURL(t, ts.URL+"/api/v1/activity", nil)
	if code != 200 {
		t.Fatalf("activity = %d, want 200 (公开)", code)
	}
	var out struct {
		Items []track.Activity `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("items = %d, want 2: %+v", len(out.Items), out.Items)
	}
	if out.Items[0].Action != "compare" || len(out.Items[0].Codes) != 2 {
		t.Errorf("首条应为合并的对比: %+v", out.Items[0])
	}
	// 响应中不得出现访客 IP
	if strings.Contains(string(body), "203.0.113") {
		t.Error("动态接口泄漏了访客 IP")
	}
}

// debateLLMMock 按角色返回辩论内容 (与 debate 包测试同款)。
func debateLLMMock(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var content string
		switch {
		case strings.Contains(string(b), "老多"):
			content = "多方发言。"
		case strings.Contains(string(b), "老空"):
			content = "空方发言。"
		case strings.Contains(string(b), "裁判"):
			content = `{"winner":"bull","bull_score":70,"bear_score":55,"reasoning":"多方更扎实。"}`
		default:
			content = "?"
		}
		c, _ := json.Marshal(content)
		io.WriteString(w, `{"choices":[{"message":{"content":`+string(c)+`}}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestDebate 验证辩论接口: 五轮结构、裁决、缓存。
func TestDebate(t *testing.T) {
	setupMocks(t) // 东财数据 mock
	cfg := testConfig()
	cfg.LLM.BaseURL = debateLLMMock(t).URL
	srv, err := New(cfg, Options{CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	code, body := getURL(t, ts.URL+"/api/v1/debate?code=600519", nil)
	if code != 200 {
		t.Fatalf("debate = %d: %s", code, body)
	}
	var d struct {
		Name  string `json:"name"`
		Turns []struct {
			Role  string `json:"role"`
			Round int    `json:"round"`
		} `json:"turns"`
		Verdict struct {
			Winner    string `json:"winner"`
			BullScore int    `json:"bull_score"`
		} `json:"verdict"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Turns) != 4 || d.Turns[0].Role != "bull" || d.Turns[1].Role != "bear" {
		t.Fatalf("辩论轮次异常: %s", body[:200])
	}
	if d.Verdict.Winner != "bull" || d.Verdict.BullScore != 70 {
		t.Errorf("裁决异常: %+v", d.Verdict)
	}
	if d.Name != "贵州茅台" {
		t.Errorf("名称 = %q, want 贵州茅台", d.Name)
	}

	// 第二次应命中缓存
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/debate?code=600519", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.Header.Get("X-Cache") != "hit" {
		t.Error("第二次辩论请求应命中缓存")
	}
}

// TestSignalRecording 验证分析成功后自动记录 AI 信号 (缓存命中不重复记)。
func TestSignalRecording(t *testing.T) {
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

	getURL(t, ts.URL+"/api/v1/analyze?code=600519", nil)
	getURL(t, ts.URL+"/api/v1/analyze?code=600519", nil) // 缓存命中，不应重复记

	deadline := time.Now().Add(3 * time.Second)
	var sigs []track.Signal
	for time.Now().Before(deadline) {
		sigs, err = tr.RecentSignals(10)
		if err != nil {
			t.Fatal(err)
		}
		if len(sigs) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(sigs) != 1 {
		t.Fatalf("信号 = %d, want 1 (缓存命中不重复记)", len(sigs))
	}
	if sigs[0].Code != "600519" || sigs[0].Signal != "bullish" || sigs[0].Price != 1253 {
		t.Errorf("信号内容异常: %+v", sigs[0])
	}
}

// TestNewsEndpoint 验证宏观快讯: 解析与 5 分钟缓存 (上游只打一次)。
func TestNewsEndpoint(t *testing.T) {
	var hits atomic.Int32
	newsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		io.WriteString(w, `{"result":{"status":{"code":0},"data":{"feed":{"list":[
			{"id":1,"rich_text":"【证监会召开发布会】回应市场关切。","create_time":"2026-07-21 19:22:54"}]}}}}`)
	}))
	defer newsSrv.Close()
	old := news.FeedURL
	news.FeedURL = newsSrv.URL
	defer func() { news.FeedURL = old }()

	srv, err := New(testConfig(), Options{CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	code, body := getURL(t, ts.URL+"/api/v1/news", nil)
	if code != 200 {
		t.Fatalf("news = %d: %s", code, body)
	}
	var out struct {
		Items []struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Title != "证监会召开发布会" {
		t.Fatalf("快讯解析异常: %s", body)
	}
	getURL(t, ts.URL+"/api/v1/news", nil)
	if got := hits.Load(); got != 1 {
		t.Errorf("缓存生效期上游被重复调用: %d 次", got)
	}
}

// capitalMockServer 按路径/报表名路由: fflow → 资金流; RZRQ → 两融; MUTUAL → 北向。
func capitalMockServer(t *testing.T, failNorth bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("reportName")
		switch {
		case q == "": // fflow 请求无 reportName 参数
			io.WriteString(w, `{"rc":0,"data":{"klines":["2026-07-21,-579159136,-367185,579526224,-484363808,-94795328"]}}`)
		case strings.Contains(q, "RZRQ"):
			io.WriteString(w, `{"success":true,"result":{"data":[{"DATE":"2026-07-20 00:00:00","RZYE":18189764566,"RQYE":159313275,"RZRQYE":18349077841,"RZMRE":1099058777,"RZYEZB":1.0961,"RZMRE5D":3418179149}]}}`)
		case strings.Contains(q, "MUTUAL"):
			if failNorth {
				io.WriteString(w, `{"success":false,"result":null}`)
				return
			}
			io.WriteString(w, `{"success":true,"result":{"data":[{"TRADE_DATE":"2026-06-30 00:00:00","HOLD_SHARES":53711656,"HOLD_MARKET_CAP":63674631071.44,"HOLD_SHARES_RATIO":4.29}]}}`)
		default:
			io.WriteString(w, `{"success":false,"result":null}`)
		}
	}))
	t.Cleanup(srv.Close)
	oldDC, oldFF := eastmoney.DataCenterURL, eastmoney.FFlowBaseURL
	eastmoney.DataCenterURL, eastmoney.FFlowBaseURL = srv.URL, srv.URL
	t.Cleanup(func() { eastmoney.DataCenterURL, eastmoney.FFlowBaseURL = oldDC, oldFF })
	return srv
}

// TestCapitalEndpoint 验证资金面板: 三项数据、部分失败降级、缓存与参数校验。
func TestCapitalEndpoint(t *testing.T) {
	capitalMockServer(t, true) // 北向失败, 验证降级
	srv, err := New(testConfig(), Options{CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	if code, _ := getURL(t, ts.URL+"/api/v1/capital?code=abc", nil); code != 400 {
		t.Errorf("非法代码 = %d, want 400", code)
	}

	code, body := getURL(t, ts.URL+"/api/v1/capital?code=600519", nil)
	if code != 200 {
		t.Fatalf("capital = %d: %s", code, body)
	}
	var out struct {
		Margin *struct {
			RZYE float64 `json:"rzye"`
		} `json:"margin"`
		FundFlow []struct {
			Main float64 `json:"main"`
		} `json:"fund_flow"`
		Northbound *struct {
			Shares float64 `json:"shares"`
		} `json:"northbound"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Margin == nil || out.Margin.RZYE != 18189764566 {
		t.Errorf("两融缺失: %s", body)
	}
	if len(out.FundFlow) != 1 || out.FundFlow[0].Main != -579159136 {
		t.Errorf("资金流缺失: %s", body)
	}
	if out.Northbound != nil {
		t.Errorf("北向失败应降级为 null: %s", body)
	}
}
