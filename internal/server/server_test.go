package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/eastmoney"
)

var llmCalls atomic.Int32

func setupMocks(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/kline", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, klineFixture(120))
	})
	mux.HandleFunc("/quote", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"rc":0,"data":{"f43":125300,"f57":"600519","f58":"贵州茅台","f60":125899,`+
			`"f116":1.5e+12,"f117":1.5e+12,"f162":1437,"f167":664,"f168":47,"f170":-48}}`)
	})
	mux.HandleFunc("/ann", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"list":[{"title":"贵州茅台:重大事项公告","notice_date":"2026-07-18 00:00:00"}]}}`)
	})
	emSrv := httptest.NewServer(mux)
	t.Cleanup(emSrv.Close)

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmCalls.Add(1)
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "四位分析师") {
			io.WriteString(w, `{"choices":[{"message":{"content":`+
				`"{\"signal\":\"bullish\",\"confidence\":70,\"summary\":\"综合偏多。\",\"key_points\":[\"要点一\"]}"}}]}`)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":`+
			`"{\"signal\":\"bullish\",\"confidence\":75,\"reasoning\":\"指标走强。\"}"}}]}`)
	}))
	t.Cleanup(llmSrv.Close)

	for p, v := range map[*string]string{
		&eastmoney.KlineBaseURL: emSrv.URL + "/kline",
		&eastmoney.QuoteBaseURL: emSrv.URL + "/quote",
		&eastmoney.AnnBaseURL:   emSrv.URL + "/ann",
	} {
		old := *p
		*p = v
		t.Cleanup(func() { *p = old })
	}
	return llmSrv
}

func newTestServer(t *testing.T, llmURL string) *httptest.Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.LLM.BaseURL = llmURL
	cfg.LLM.APIKey = "k"
	cfg.LLM.Model = "m"
	srv, err := New(cfg, time.Minute, "")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestHealth(t *testing.T) {
	ts := newTestServer(t, "http://unused")
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz = %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("缺少 CORS 头")
	}
}

func TestAnalyzeAndCache(t *testing.T) {
	llmSrv := setupMocks(t)
	ts := newTestServer(t, llmSrv.URL)
	llmCalls.Store(0)

	resp, err := http.Get(ts.URL + "/api/v1/analyze?code=600519")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("analyze = %d: %s", resp.StatusCode, body)
	}
	var jr struct {
		Name  string `json:"name"`
		Final struct {
			Signal string `json:"signal"`
		} `json:"final"`
	}
	if err := json.Unmarshal(body, &jr); err != nil {
		t.Fatalf("响应非 JSON: %v", err)
	}
	if jr.Name != "贵州茅台" || jr.Final.Signal != "bullish" {
		t.Errorf("响应内容异常: %s", body[:200])
	}
	firstCalls := llmCalls.Load()
	if firstCalls != 5 {
		t.Errorf("LLM 调用 = %d, want 5 (4 分析师 + 1 组合经理)", firstCalls)
	}

	// 第二次请求应命中缓存，不再调 LLM
	resp2, err := http.Get(ts.URL + "/api/v1/analyze?code=600519")
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.Header.Get("X-Cache") != "hit" {
		t.Error("第二次请求应命中缓存 (X-Cache: hit)")
	}
	if got := llmCalls.Load(); got != firstCalls {
		t.Errorf("缓存命中后 LLM 调用 = %d, want %d (不变)", got, firstCalls)
	}
}

func TestAnalyzeBadRequests(t *testing.T) {
	ts := newTestServer(t, "http://unused")
	for _, tc := range []struct {
		url  string
		want int
	}{
		{"/api/v1/analyze", 400},                         // 缺 code
		{"/api/v1/analyze?code=123", 400},                // 代码格式错误
		{"/api/v1/analyze?code=600519&source=sina", 400}, // 未知数据源
	} {
		resp, err := http.Get(ts.URL + tc.url)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("GET %s = %d, want %d: %s", tc.url, resp.StatusCode, tc.want, body)
		}
		if !strings.Contains(string(body), `"error"`) {
			t.Errorf("GET %s 响应应包含 error 字段: %s", tc.url, body)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	ts := newTestServer(t, "http://unused")
	resp, err := http.Post(ts.URL+"/api/v1/analyze?code=600519", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d, want 405", resp.StatusCode)
	}
}

// klineFixture 生成 n 根单调递增的日 K 线。
func klineFixture(n int) string {
	var b strings.Builder
	b.WriteString(`{"rc":0,"data":{"code":"600519","market":1,"name":"贵州茅台","klines":[`)
	price := 1000.0
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		price++
		fmt.Fprintf(&b, `"2026-03-%02d,%.2f,%.2f,%.2f,%.2f,1000"`, i%28+1, price-0.5, price, price+1, price-1)
	}
	b.WriteString(`]}}`)
	return b.String()
}

// TestAccessLog 验证访问记录: XFF 解析、缓存命中标记、查询接口、JSONL 落盘。
func TestAccessLog(t *testing.T) {
	llmSrv := setupMocks(t)
	cfg := &config.Config{}
	cfg.LLM.BaseURL = llmSrv.URL
	cfg.LLM.APIKey = "k"
	cfg.LLM.Model = "m"
	logPath := filepath.Join(t.TempDir(), "access.jsonl")
	srv, err := New(cfg, time.Minute, logPath)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 第一次请求 (带 X-Forwarded-For) + 第二次缓存请求
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/analyze?code=600519&source=ths", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()

	resp2, err := http.Get(ts.URL + "/api/v1/analyze?code=600519&source=ths")
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp2.Body)
	resp2.Body.Close()

	// 查询接口
	aresp, err := http.Get(ts.URL + "/api/v1/access-log?limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer aresp.Body.Close()
	var out struct {
		Records []AccessRecord `json:"records"`
	}
	if err := json.NewDecoder(aresp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(out.Records))
	}
	// 新的在前: 第一条应为缓存命中
	if !out.Records[0].CacheHit {
		t.Error("最新记录应为缓存命中")
	}
	first := out.Records[1]
	if first.IP != "203.0.113.7" {
		t.Errorf("IP = %q, want XFF 首个地址", first.IP)
	}
	if first.Code != "600519" || first.Source != "ths" || first.Status != 200 || first.CacheHit {
		t.Errorf("首条记录内容异常: %+v", first)
	}

	// JSONL 落盘
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(string(data)), "\n"); len(lines) != 2 {
		t.Errorf("JSONL 应为 2 行, 实际 %d", len(lines))
	}
}
