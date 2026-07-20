package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/eastmoney"
	"github.com/deng23yu/stockagent/internal/ths"
)

func TestRun(t *testing.T) {
	setupMocks(t)
	cfg := &config.Config{}
	cfg.LLM.BaseURL = mockLLMURL
	cfg.LLM.APIKey = "k"
	cfg.LLM.Model = "m"

	data, err := Run(context.Background(), cfg, "600519", Options{Days: 120}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if data.Name != "贵州茅台" || data.Code != "600519" {
		t.Errorf("报告主体异常: %s (%s)", data.Name, data.Code)
	}
	if len(data.Results) != 4 {
		t.Errorf("分析师数量 = %d, want 4", len(data.Results))
	}
	if data.Final.Summary == "" {
		t.Error("组合经理结论为空")
	}
}

func TestRunInputErrors(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.APIKey = "k"
	for _, tc := range []struct {
		name   string
		code   string
		source string
	}{
		{"代码位数错误", "12345", ""},
		{"代码含字母", "60a519", ""},
		{"未知数据源", "600519", "sina"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Run(context.Background(), cfg, tc.code, Options{Source: tc.source}, nil)
			var ie *InputError
			if !errors.As(err, &ie) {
				t.Fatalf("err = %v, want InputError", err)
			}
		})
	}
}

func TestRunMissingAPIKey(t *testing.T) {
	_, err := Run(context.Background(), &config.Config{}, "600519", Options{}, nil)
	if err == nil || !strings.Contains(err.Error(), "API Key") {
		t.Fatalf("err = %v, want 缺少 API Key 提示", err)
	}
}

var mockLLMURL string

// setupMocks 拉起 mock 的东财与 LLM 服务并替换包级 URL。
func setupMocks(t *testing.T) {
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
	mockLLMURL = llmSrv.URL

	for p, v := range map[*string]string{
		&eastmoney.KlineBaseURL: emSrv.URL + "/kline",
		&eastmoney.QuoteBaseURL: emSrv.URL + "/quote",
		&eastmoney.AnnBaseURL:   emSrv.URL + "/ann",
	} {
		old := *p
		*p = v
		t.Cleanup(func() { *p = old })
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

// TestRunKlineFallbackToTHS 验证东财 K 线失败时自动回退同花顺 K 线。
func TestRunKlineFallbackToTHS(t *testing.T) {
	setupMocks(t)

	// 东财 K 线改为直接断开的死服务
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("不支持 hijack")
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	t.Cleanup(dead.Close)
	oldKline := eastmoney.KlineBaseURL
	eastmoney.KlineBaseURL = dead.URL + "/kline"
	t.Cleanup(func() { eastmoney.KlineBaseURL = oldKline })

	// ths K 线 mock (JSONP 包装, data 为分号分隔记录)
	thsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `cb({"data":"`+thsKlineRecords(120)+`"})`)
	}))
	t.Cleanup(thsSrv.Close)
	oldThs := ths.KlineBaseURL
	ths.KlineBaseURL = thsSrv.URL
	t.Cleanup(func() { ths.KlineBaseURL = oldThs })

	cfg := &config.Config{}
	cfg.LLM.BaseURL = mockLLMURL
	cfg.LLM.APIKey = "k"
	cfg.LLM.Model = "m"

	data, err := Run(context.Background(), cfg, "600519", Options{Days: 120}, nil)
	if err != nil {
		t.Fatalf("应通过 ths 回退成功: %v", err)
	}
	if data.Name != "贵州茅台" { // ths K 线无名称, 应回退快照名
		t.Errorf("名称 = %q, want 贵州茅台 (来自快照)", data.Name)
	}
}

// thsKlineRecords 生成 n 根 ths 格式的日 K 记录 (date,open,high,low,close,volume,...)。
func thsKlineRecords(n int) string {
	var b strings.Builder
	price := 1000.0
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(';')
		}
		price++
		fmt.Fprintf(&b, `202603%02d,%.2f,%.2f,%.2f,%.2f,5841730,7322732700.00,0.467,,,`,
			i%28+1, price-0.5, price, price-1, price)
	}
	return b.String()
}
