package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stockagent/internal/eastmoney"
	"stockagent/internal/indicator"
	"stockagent/internal/llm"
)

func TestExtractJSON(t *testing.T) {
	in := "好的，分析如下:\n```json\n{\"signal\":\"bullish\",\"confidence\":80}\n```\n以上。"
	want := `{"signal":"bullish","confidence":80}`
	if got := extractJSON(in); got != want {
		t.Errorf("extractJSON = %q, want %q", got, want)
	}
}

func TestParseResult(t *testing.T) {
	r, err := parseResult("测试", `{"signal":"看多","confidence":150,"reasoning":"趋势走强"}`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Signal != SignalBullish {
		t.Errorf("Signal = %q, want bullish", r.Signal)
	}
	if r.Confidence != 100 {
		t.Errorf("Confidence = %d, want 100 (截断)", r.Confidence)
	}
	if _, err := parseResult("测试", `{"signal":"bullish"}`); err == nil {
		t.Error("缺少 reasoning 应报错")
	}
	if _, err := parseResult("测试", "not json at all"); err == nil {
		t.Error("非 JSON 应报错")
	}
}

func TestNormalizeSignal(t *testing.T) {
	cases := map[string]Signal{
		"bullish": SignalBullish,
		"看空":      SignalBearish,
		"SELL":    SignalBearish,
		"neutral": SignalNeutral,
		"持有":      SignalNeutral,
	}
	for in, want := range cases {
		if got := normalizeSignal(in); got != want {
			t.Errorf("normalizeSignal(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFallbackVerdict(t *testing.T) {
	results := []Result{
		{Agent: "a", Signal: SignalBullish, Confidence: 80},
		{Agent: "b", Signal: SignalBullish, Confidence: 60},
		{Agent: "c", Signal: SignalBearish, Confidence: 70},
		{Agent: "d", Signal: SignalNeutral, Confidence: 0, Err: "failed"},
	}
	v := fallbackVerdict(results)
	// score = 80+60-70 = 70, weight = 210, avg ≈ 0.333 → bullish
	if v.Signal != SignalBullish {
		t.Errorf("Signal = %q, want bullish", v.Signal)
	}
	if !v.Degraded {
		t.Error("fallback 应标记 Degraded")
	}
	if len(v.KeyPoints) != 3 {
		t.Errorf("KeyPoints = %d, want 3 (跳过失败分析师)", len(v.KeyPoints))
	}
}

// mockLLM 按请求内容区分分析师与组合经理，返回固定 JSON。
func mockLLM(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "四位分析师") {
			io.WriteString(w, `{"choices":[{"message":{"content":"{\"signal\":\"bullish\",\"confidence\":70,\"summary\":\"综合各分析师观点，整体偏多。\",\"key_points\":[\"要点一\",\"要点二\"]}"}}]}`)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"{\"signal\":\"bullish\",\"confidence\":75,\"reasoning\":\"指标走强。\"}"}}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testContext(srvURL string) *Context {
	return &Context{
		Code: "600519",
		Name: "贵州茅台",
		Snapshot: &eastmoney.Snapshot{
			Price: 1253, PE: 14.37, PB: 6.64,
			TotalMarketCap: 1.566e12, ChangePct: -0.48, TurnoverPct: 0.47,
		},
		Indicators: indicator.Summary{
			Price: 1253, RSI14: 55, AnnualizedVol: 22, MaxDrawdown: 18,
			High52W: 1900, Low52W: 1100,
		},
		Announcements: []eastmoney.Announcement{
			{Title: "贵州茅台:重大事项公告", Date: "2026-07-18"},
		},
		LLM: llm.New(srvURL, "k", "m"),
	}
}

func TestAgentsAnalyze(t *testing.T) {
	srv := mockLLM(t)
	ctx := testContext(srv.URL)
	for _, ag := range All() {
		r := ag.Analyze(context.Background(), ctx)
		if r.Err != "" {
			t.Errorf("%s 分析失败: %s", ag.Name(), r.Err)
			continue
		}
		if r.Signal != SignalBullish || r.Confidence != 75 {
			t.Errorf("%s 结果异常: %+v", ag.Name(), r)
		}
	}
}

func TestSentimentNoAnnouncements(t *testing.T) {
	ctx := testContext("http://unused")
	ctx.Announcements = nil
	r := Sentiment{}.Analyze(context.Background(), ctx)
	if r.Signal != SignalNeutral || r.Err != "" {
		t.Errorf("无公告应返回本地中性: %+v", r)
	}
}

func TestPortfolioManagerSynthesize(t *testing.T) {
	srv := mockLLM(t)
	ctx := testContext(srv.URL)
	results := []Result{
		{Agent: "技术面分析师", Signal: SignalBullish, Confidence: 75, Reasoning: "指标走强。"},
	}
	v := PortfolioManager{}.Synthesize(context.Background(), ctx, results)
	if v.Degraded {
		t.Fatal("LLM 正常时不应降级")
	}
	if v.Signal != SignalBullish || v.Confidence != 70 || v.Summary == "" || len(v.KeyPoints) != 2 {
		t.Errorf("综合结论异常: %+v", v)
	}
}

func TestPortfolioManagerDegrade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	ctx := testContext(srv.URL)
	results := []Result{
		{Agent: "技术面分析师", Signal: SignalBullish, Confidence: 80, Reasoning: "强"},
	}
	v := PortfolioManager{}.Synthesize(context.Background(), ctx, results)
	if !v.Degraded {
		t.Error("LLM 持续 500 应降级为本地聚合")
	}
}
