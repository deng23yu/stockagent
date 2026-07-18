// mockdemo 在本地拉起 mock 的东财数据服务与 LLM 服务，
// 无需任何 API key 即可端到端演示 stockagent analyze 的完整流程。
//
// 运行: go run ./cmd/mockdemo
package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/deng23yu/stockagent/internal/cli"
	"github.com/deng23yu/stockagent/internal/eastmoney"
)

func main() {
	emSrv := httptest.NewServer(eastmoneyMux())
	defer emSrv.Close()
	llmSrv := httptest.NewServer(http.HandlerFunc(llmHandler))
	defer llmSrv.Close()

	eastmoney.KlineBaseURL = emSrv.URL + "/kline"
	eastmoney.QuoteBaseURL = emSrv.URL + "/quote"
	eastmoney.AnnBaseURL = emSrv.URL + "/ann"
	os.Setenv("STOCKAGENT_API_KEY", "demo-key")

	fmt.Println("== mockdemo: 东财与 LLM 均为本地 mock，仅演示流程 ==")
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"analyze", "600519", "--base-url", llmSrv.URL})
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "demo 执行失败:", err)
		os.Exit(1)
	}
}

// eastmoneyMux 返回 mock 的东财三个接口。
func eastmoneyMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/kline", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, klineFixture())
	})
	mux.HandleFunc("/quote", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"rc":0,"data":{"f43":125300,"f44":126800,"f45":124500,"f46":126000,`+
			`"f57":"600519","f58":"贵州茅台","f60":124800,"f116":1.566e+12,"f117":1.566e+12,`+
			`"f162":1437,"f167":664,"f168":47,"f170":40}}`)
	})
	mux.HandleFunc("/ann", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"list":[`+
			`{"title":"贵州茅台:2025年年度权益分派实施公告","notice_date":"2026-06-28 00:00:00"},`+
			`{"title":"贵州茅台:关于回购股份进展的公告","notice_date":"2026-06-15 00:00:00"},`+
			`{"title":"贵州茅台:2026年半年度业绩预告","notice_date":"2026-07-10 00:00:00"},`+
			`{"title":"贵州茅台:2025年年度股东大会决议公告","notice_date":"2026-05-30 00:00:00"}]}}`)
	})
	return mux
}

// klineFixture 生成 250 根固定种子的随机游走日 K 线，结尾略微回落。
func klineFixture() string {
	rng := rand.New(rand.NewSource(42))
	var b strings.Builder
	b.WriteString(`{"rc":0,"data":{"code":"600519","market":1,"name":"贵州茅台","klines":[`)

	day := time.Date(2025, 7, 21, 0, 0, 0, 0, time.UTC)
	close := 1480.0
	for i := 0; i < 250; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			day = day.AddDate(0, 0, 1)
		}
		drift := -0.9 + rng.Float64()*2.2 // 整体缓慢下行，含波动
		if i > 235 {
			drift -= 1.5 // 尾部回落
		}
		close += drift
		open := close + (rng.Float64()-0.5)*10
		high := max(open, close) + rng.Float64()*8
		low := min(open, close) - rng.Float64()*8
		vol := 20000 + rng.Intn(40000)
		fmt.Fprintf(&b, `"%s,%.2f,%.2f,%.2f,%.2f,%d"`,
			day.Format("2006-01-02"), open, close, high, low, vol)
		day = day.AddDate(0, 0, 1)
	}
	b.WriteString(`]}}`)
	return b.String()
}

// llmHandler 按 prompt 特征区分四位分析师与组合经理，返回预置结论。
func llmHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	prompt := string(body)

	reply := `{"signal":"neutral","confidence":50,"reasoning":"数据不足，维持中性。"}`
	switch {
	case strings.Contains(prompt, "四位分析师"):
		reply = `{"signal":"bullish","confidence":62,"summary":"技术面与消息面偏多，估值处于合理区间，风控提示短期动能不足。综合看当前位置具备一定配置价值，但缺乏强催化，建议分批布局、控制仓位。","key_points":["MA20 上穿 MA60，中期趋势转强","PE 14.4 倍处于历史中枢下方，估值合理","分红与回购释放积极信号，无重大利空","波动温和但短期动能不足，宜分批介入"]}`
	case strings.Contains(prompt, "技术面分析"):
		reply = `{"signal":"bullish","confidence":68,"reasoning":"MA20 上穿 MA60 形成金叉，MACD 红柱放大，中期趋势转强；RSI 62 未进入超买区，量能温和放大，技术面整体偏多。"}`
	case strings.Contains(prompt, "基本面估值角度"):
		reply = `{"signal":"neutral","confidence":55,"reasoning":"动态 PE 14.4 倍处于白酒板块历史中枢下方，PB 6.6 倍合理，市值大、换手率低，估值无明显高估但也缺乏弹性。"}`
	case strings.Contains(prompt, "消息面分析"):
		reply = `{"signal":"bullish","confidence":60,"reasoning":"近期公告以权益分派实施、回购进展和业绩预告为主，释放积极信号，无利空事项，消息面温和偏多。"}`
	case strings.Contains(prompt, "风险控制角度"):
		reply = `{"signal":"neutral","confidence":52,"reasoning":"年化波动率约 22% 处于中等水平，现价距 52 周高点回撤约两成，追高风险有限，但短期上行动能不足，风险收益比中性。"}`
	}
	fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%s}}]}`,
		jsonString(reply))
}

// jsonString 将 s 包装为 JSON 字符串字面量。
func jsonString(s string) string {
	q := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + q.Replace(s) + `"`
}
