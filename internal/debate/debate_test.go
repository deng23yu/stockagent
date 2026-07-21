package debate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deng23yu/stockagent/internal/agent"
	"github.com/deng23yu/stockagent/internal/eastmoney"
	"github.com/deng23yu/stockagent/internal/indicator"
	"github.com/deng23yu/stockagent/internal/llm"
)

// mockLLM 按 system prompt 中的角色返回对应内容。
func mockLLM(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		var content string
		switch {
		case strings.Contains(body, "老多"):
			content = "多方: 业绩稳健估值低，上涨空间打开。"
		case strings.Contains(body, "老空"):
			content = "空方: 增长乏力风险积聚，反弹是出货。"
		case strings.Contains(body, "裁判"):
			content = `{"winner":"bull","bull_score":72,"bear_score":58,"reasoning":"多方数据支撑更扎实。"}`
		default:
			content = "???"
		}
		c, _ := json.Marshal(content)
		io.WriteString(w, `{"choices":[{"message":{"content":`+string(c)+`}}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testContext(llmURL string) *agent.Context {
	return &agent.Context{
		Code: "600519",
		Name: "贵州茅台",
		Snapshot: &eastmoney.Snapshot{
			Code: "600519", Name: "贵州茅台", Price: 1253.0, ChangePct: 1.2,
			PE: 14.4, PB: 6.6, TotalMarketCap: 1.5e12,
		},
		Indicators: indicator.Summary{ChangePct5: 2.1, ChangePct20: 5.3, MA5: 1250, MA20: 1230, RSI14: 58},
		Bars:       []eastmoney.Kline{{Date: "2026-07-17", Close: 1240}, {Date: "2026-07-20", Close: 1253}},
		LLM:        llm.New(llmURL, "k", "m", nil),
	}
}

func TestRun(t *testing.T) {
	d, err := Run(context.Background(), testContext(mockLLM(t).URL))
	if err != nil {
		t.Fatal(err)
	}
	// 5 轮: 多-空-多-空-裁判
	if len(d.Turns) != 4 {
		t.Fatalf("turns = %d, want 4", len(d.Turns))
	}
	wantRoles := []string{"bull", "bear", "bull", "bear"}
	for i, tr := range d.Turns {
		if tr.Role != wantRoles[i] || tr.Round != i/2+1 {
			t.Errorf("第 %d 轮顺序异常: %+v", i, tr)
		}
		if tr.Content == "" || tr.Content == "???" {
			t.Errorf("第 %d 轮内容为空: %+v", i, tr)
		}
	}
	if d.Verdict.Winner != "bull" || d.Verdict.BullScore != 72 || d.Verdict.BearScore != 58 {
		t.Errorf("裁决异常: %+v", d.Verdict)
	}
	if d.Name != "贵州茅台" || d.Code != "600519" {
		t.Errorf("标的异常: %s %s", d.Name, d.Code)
	}
}

func TestParseVerdictFallback(t *testing.T) {
	v := parseVerdict("裁判脑子进水了没有 JSON")
	if v.Winner != "draw" || v.BullScore != 50 || v.BearScore != 50 {
		t.Errorf("垃圾输入应降级为平局: %+v", v)
	}
	v = parseVerdict(`前言 {"winner":"bear","bull_score":40,"bear_score":75,"reasoning":"空方逻辑更闭环。"} 后记`)
	if v.Winner != "bear" || v.BearScore != 75 {
		t.Errorf("包裹文本中的 JSON 应能解析: %+v", v)
	}
}
