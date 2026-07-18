package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestStaticUI 验证内嵌前端: 根路径返回 HTML, 未知路径 SPA fallback, API 不受影响。
func TestStaticUI(t *testing.T) {
	ts := newTestServer(t, "http://unused")

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET / = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(string(body), "stockagent") {
		t.Error("首页应包含 stockagent 标识")
	}

	// SPA fallback: 未知路径同样返回 index.html
	resp2, err := http.Get(ts.URL + "/some/unknown/route")
	if err != nil {
		t.Fatal(err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != 200 || !strings.Contains(string(body2), "stockagent") {
		t.Error("SPA fallback 失效")
	}

	// API 路由不受静态服务影响
	resp3, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if !strings.Contains(string(body3), `"status":"ok"`) {
		t.Errorf("healthz 被静态服务干扰: %s", body3)
	}
}
