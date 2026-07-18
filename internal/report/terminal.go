package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/olekukonko/tablewriter"

	"stockagent/internal/agent"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiGray   = "\033[90m"
)

func paint(s, code string, enabled bool) string {
	if !enabled {
		return s
	}
	return code + s + ansiReset
}

// paintSignal 按 A 股习惯着色: 红涨(看多)绿跌(看空)。
func paintSignal(sig agent.Signal, text string, enabled bool) string {
	switch sig {
	case agent.SignalBullish:
		return paint(text, ansiRed, enabled)
	case agent.SignalBearish:
		return paint(text, ansiGreen, enabled)
	default:
		return paint(text, ansiYellow, enabled)
	}
}

// RenderTerminal 渲染终端报告。
func RenderTerminal(w io.Writer, d *Data, color bool) {
	s := d.Snapshot
	line := strings.Repeat("─", 56)

	fmt.Fprintln(w)
	fmt.Fprintln(w, paint(fmt.Sprintf("%s (%s) · AI 投研报告", d.Name, d.Code), ansiBold, color))
	fmt.Fprintln(w, paint(line, ansiGray, color))

	priceText := fmt.Sprintf("现价 %.2f  %+.2f%%", s.Price, s.ChangePct)
	if s.ChangePct >= 0 {
		priceText = paint(priceText, ansiRed, color)
	} else {
		priceText = paint(priceText, ansiGreen, color)
	}
	fmt.Fprintf(w, "%s    PE %s  PB %.2f  总市值 %s\n",
		priceText, peText(s.PE), s.PB, agent.FormatYi(s.TotalMarketCap))
	fmt.Fprintln(w, paint(fmt.Sprintf("生成于 %s · 模型 %s · 耗时 %s",
		d.GeneratedAt.Format("2006-01-02 15:04"), d.Model, d.Elapsed), ansiGray, color))
	fmt.Fprintln(w)

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"智能体", "信号", "置信度"})
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetRowLine(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	for _, r := range d.Results {
		sig := paintSignal(r.Signal, r.Signal.CN(), color)
		if r.Err != "" {
			sig = paint("失败", ansiGray, color)
		}
		table.Append([]string{r.Agent, sig, fmt.Sprintf("%d", r.Confidence)})
	}
	table.Render()
	fmt.Fprintln(w)

	final := fmt.Sprintf("综合结论: %s (置信度 %d)", d.Final.Signal.CN(), d.Final.Confidence)
	fmt.Fprintln(w, paintSignal(d.Final.Signal, paint(final, ansiBold, color), color))
	if d.Final.Degraded {
		fmt.Fprintln(w, paint("(LLM 组合经理不可用，已降级为本地聚合)", ansiGray, color))
	}
	fmt.Fprintln(w, paint(line, ansiGray, color))
	fmt.Fprintln(w, d.Final.Summary)

	if len(d.Final.KeyPoints) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, paint("关键要点:", ansiBold, color))
		for i, p := range d.Final.KeyPoints {
			fmt.Fprintf(w, "  %d. %s\n", i+1, p)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, paint("分析师详情:", ansiBold, color))
	for _, r := range d.Results {
		fmt.Fprintf(w, "  [%s %d] %s: %s\n", r.Signal.CN(), r.Confidence, r.Agent, r.Reasoning)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, paint("风险提示: "+Disclaimer, ansiGray, color))
}
