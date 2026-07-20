package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/track"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.LLM.BaseURL = "http://unused"
	cfg.LLM.APIKey = "k"
	cfg.LLM.Model = "m"
	return cfg
}

func newVisitsTestServer(t *testing.T, opts Options) (*httptest.Server, *track.Tracker) {
	t.Helper()
	tr, err := track.Open(filepath.Join(t.TempDir(), "visits.db"), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	opts.Tracker = tr
	if opts.CacheTTL == 0 {
		opts.CacheTTL = time.Minute
	}
	srv, err := New(testConfig(), opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() }) // 关闭 tracker，避免后台 goroutine 泄漏写已删除的临时库
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, tr
}

func getURL(t *testing.T, url string, headers map[string]string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// waitVisit 轮询直到满足条件的记录落库 (记录是异步批量写入)。
func waitVisit(t *testing.T, tr *track.Tracker, f track.Filter, match func(track.Visit) bool) []track.Visit {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		recs, err := tr.QueryVisits(100, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range recs {
			if match == nil || match(v) {
				return recs
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("等待落库超时")
	return nil
}

// TestVisitsTracking 验证访客记录: 页面与 API 请求落库、静态资源/健康检查跳过、
// 管理 token 不写入库、/api/v1/visits 查询接口。
func TestVisitsTracking(t *testing.T) {
	ts, tr := newVisitsTestServer(t, Options{})

	getURL(t, ts.URL+"/", nil)                                   // 页面访问 → 记录
	getURL(t, ts.URL+"/assets/app.js", nil)                      // 静态资源 → 跳过
	getURL(t, ts.URL+"/healthz", nil)                            // 健康检查 → 跳过
	getURL(t, ts.URL+"/api/v1/visits?limit=5&token=secret", nil) // 查询本身也被记录，但 token 不落库

	recs := waitVisit(t, tr, track.Filter{}, func(v track.Visit) bool { return v.Path == "/" })
	var page *track.Visit
	for i := range recs {
		v := &recs[i]
		switch {
		case v.Path == "/":
			page = v
		case v.Path == "/healthz" || strings.HasPrefix(v.Path, "/assets/"):
			t.Errorf("%s 不应被记录", v.Path)
		case strings.Contains(v.Query, "token"):
			t.Errorf("管理 token 不应落库: %q", v.Query)
		}
	}
	if page == nil {
		t.Error("GET / 应被记录")
	}

	// 查询接口
	_, body := getURL(t, ts.URL+"/api/v1/visits?limit=100", nil)
	var out struct {
		Records []track.Visit `json:"records"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Records) == 0 {
		t.Error("/api/v1/visits 应返回记录")
	}
}

// TestAdminToken 验证管理接口鉴权: 无/错 token 401，正确 token (Header 与 query 两种) 200。
func TestAdminToken(t *testing.T) {
	ts, _ := newVisitsTestServer(t, Options{AdminToken: "s3cret"})

	for _, u := range []string{"/api/v1/visits", "/api/v1/visits/stats"} {
		if code, _ := getURL(t, ts.URL+u, nil); code != 401 {
			t.Errorf("GET %s 无 token = %d, want 401", u, code)
		}
		if code, _ := getURL(t, ts.URL+u, map[string]string{"Authorization": "Bearer wrong"}); code != 401 {
			t.Errorf("GET %s 错 token = %d, want 401", u, code)
		}
		if code, _ := getURL(t, ts.URL+u, map[string]string{"Authorization": "Bearer s3cret"}); code != 200 {
			t.Errorf("GET %s Bearer token = %d, want 200", u, code)
		}
		if code, _ := getURL(t, ts.URL+u+"?token=s3cret", nil); code != 200 {
			t.Errorf("GET %s ?token= = %d, want 200", u, code)
		}
	}
	// analyze 不受鉴权影响 (400 而非 401)
	if code, _ := getURL(t, ts.URL+"/api/v1/analyze", nil); code != 400 {
		t.Errorf("analyze 不需要 token, = %d, want 400", code)
	}
}

// TestTrustProxy 验证访客 IP 提取: 默认不信 XFF (用 RemoteAddr)，开启后取 XFF 首个地址。
func TestTrustProxy(t *testing.T) {
	// 默认: TrustProxy=false，XFF 应被忽略
	ts, tr := newVisitsTestServer(t, Options{})
	getURL(t, ts.URL+"/", map[string]string{"X-Forwarded-For": "203.0.113.7"})
	waitVisit(t, tr, track.Filter{Path: "/"}, func(v track.Visit) bool {
		if v.IP != "127.0.0.1" {
			t.Fatalf("TrustProxy=false 时 IP = %q, want RemoteAddr (127.0.0.1)", v.IP)
		}
		return true
	})

	// 开启: 取 XFF
	ts2, tr2 := newVisitsTestServer(t, Options{TrustProxy: true})
	getURL(t, ts2.URL+"/", map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.1"})
	waitVisit(t, tr2, track.Filter{Path: "/"}, func(v track.Visit) bool {
		if v.IP != "203.0.113.7" {
			t.Fatalf("TrustProxy=true 时 IP = %q, want XFF 首个地址", v.IP)
		}
		return true
	})
}

// TestVisitsStats 验证聚合统计接口。
func TestVisitsStats(t *testing.T) {
	ts, tr := newVisitsTestServer(t, Options{})

	getURL(t, ts.URL+"/", nil)
	getURL(t, ts.URL+"/", nil)
	// 直接写入一条带 code 的记录 (analyze 会走真实上游，单测不依赖外部服务)
	tr.Record(track.Visit{Time: time.Now(), IP: "127.0.0.1", Method: "GET", Path: "/api/v1/analyze", Code: "600519", Status: 200})
	waitVisit(t, tr, track.Filter{}, func(v track.Visit) bool { return v.Code == "600519" })

	_, body := getURL(t, ts.URL+"/api/v1/visits/stats", nil)
	var stats track.Stats
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatal(err)
	}
	if stats.TotalPV < 3 || stats.TotalUV != 1 {
		t.Errorf("PV/UV 异常: %+v", stats)
	}
	if stats.TodayPV != stats.TotalPV {
		t.Errorf("今日 PV 应等于累计 PV: %+v", stats)
	}
	foundCode := false
	for _, c := range stats.TopCodes {
		if c.Name == "600519" {
			foundCode = true
		}
	}
	if !foundCode {
		t.Errorf("TopCodes 应包含 600519: %+v", stats.TopCodes)
	}
	if len(stats.TopCities) == 0 || stats.TopCities[0].Name != "内网" {
		t.Errorf("TopCities 异常: %+v", stats.TopCities)
	}
}
