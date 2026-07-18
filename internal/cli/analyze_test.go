package cli

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deng23yu/stockagent/internal/eastmoney"
)

// buildKlineFixture 生成 120 根单调递增的日 K 线。
func buildKlineFixture() string {
	var b strings.Builder
	b.WriteString(`{"rc":0,"data":{"code":"600519","market":1,"name":"贵州茅台","klines":[`)
	price := 1000.0
	for i := 0; i < 120; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		price++
		fmt.Fprintf(&b, `"2026-03-%02d,%.2f,%.2f,%.2f,%.2f,1000"`, i%28+1, price-0.5, price, price+1, price-1)
	}
	b.WriteString(`]}}`)
	return b.String()
}

// TestAnalyzeEndToEnd 用 mock 的东财与 LLM 服务跑通完整 analyze 流程。
func TestAnalyzeEndToEnd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/kline", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, buildKlineFixture())
	})
	mux.HandleFunc("/quote", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"rc":0,"data":{"f43":125300,"f57":"600519","f58":"贵州茅台","f60":125899,`+
			`"f116":1.5e+12,"f117":1.5e+12,"f162":1437,"f167":664,"f168":47,"f170":-48}}`)
	})
	mux.HandleFunc("/ann", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"list":[{"title":"贵州茅台:重大事项公告","notice_date":"2026-07-18 00:00:00"}]}}`)
	})
	emSrv := httptest.NewServer(mux)
	defer emSrv.Close()

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "四位分析师") {
			io.WriteString(w, `{"choices":[{"message":{"content":`+
				`"{\"signal\":\"bullish\",\"confidence\":70,\"summary\":\"综合偏多。\",\"key_points\":[\"要点一\"]}"}}]}`)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":`+
			`"{\"signal\":\"bullish\",\"confidence\":75,\"reasoning\":\"指标走强。\"}"}}]}`)
	}))
	defer llmSrv.Close()

	for p, v := range map[*string]string{
		&eastmoney.KlineBaseURL: emSrv.URL + "/kline",
		&eastmoney.QuoteBaseURL: emSrv.URL + "/quote",
		&eastmoney.AnnBaseURL:   emSrv.URL + "/ann",
	} {
		old := *p
		*p = v
		defer func() { *p = old }()
	}
	t.Setenv("STOCKAGENT_API_KEY", "test-key")

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"analyze", "600519", "--base-url", llmSrv.URL})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("analyze 执行失败: %v", err)
	}
	s := out.String()
	for _, want := range []string{
		"贵州茅台 (600519) · AI 投研报告",
		"综合结论: 看多 (置信度 70)",
		"要点一",
		"技术面分析师",
		"风险提示",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("输出缺少 %q\n完整输出:\n%s", want, s)
		}
	}
}

func TestAnalyzeMissingAPIKey(t *testing.T) {
	t.Setenv("STOCKAGENT_API_KEY", "")
	cmd := NewRootCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"analyze", "600519"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("未配置 API Key 时应报错")
	}
}
