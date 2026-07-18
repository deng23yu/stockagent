package report

import (
	"fmt"
	"io"

	"stockagent/internal/agent"
)

// RenderMarkdown 渲染 Markdown 报告。
func RenderMarkdown(w io.Writer, d *Data) {
	s := d.Snapshot
	fmt.Fprintf(w, "# %s (%s) AI 投研报告\n\n", d.Name, d.Code)
	fmt.Fprintf(w, "> 生成于 %s · 模型 `%s`\n\n", d.GeneratedAt.Format("2006-01-02 15:04"), d.Model)
	fmt.Fprintf(w, "- 现价: **%.2f** (%+.2f%%)\n", s.Price, s.ChangePct)
	fmt.Fprintf(w, "- PE(动态): %s | PB: %.2f | 总市值: %s\n\n",
		peText(s.PE), s.PB, agent.FormatYi(s.TotalMarketCap))

	fmt.Fprintln(w, "| 智能体 | 信号 | 置信度 |")
	fmt.Fprintln(w, "| --- | --- | --- |")
	for _, r := range d.Results {
		sig := r.Signal.CN()
		if r.Err != "" {
			sig = "失败"
		}
		fmt.Fprintf(w, "| %s | %s | %d |\n", r.Agent, sig, r.Confidence)
	}

	fmt.Fprintf(w, "\n## 综合结论: %s (置信度 %d)\n\n", d.Final.Signal.CN(), d.Final.Confidence)
	if d.Final.Degraded {
		fmt.Fprintln(w, "> LLM 组合经理不可用，本结论为本地加权聚合。")
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%s\n", d.Final.Summary)

	if len(d.Final.KeyPoints) > 0 {
		fmt.Fprint(w, "\n### 关键要点\n\n")
		for i, p := range d.Final.KeyPoints {
			fmt.Fprintf(w, "%d. %s\n", i+1, p)
		}
	}

	fmt.Fprint(w, "\n### 分析师详情\n\n")
	for _, r := range d.Results {
		fmt.Fprintf(w, "- **%s** (%s, %d): %s\n", r.Agent, r.Signal.CN(), r.Confidence, r.Reasoning)
	}
	fmt.Fprintf(w, "\n---\n\n> **风险提示**: %s\n", Disclaimer)
}
