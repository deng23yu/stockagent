package agent

import (
	"context"
	"fmt"
	"strings"
)

// Sentiment 消息面分析师: 基于近期公告标题给出情绪信号。
type Sentiment struct{}

// Name 返回分析师名称。
func (Sentiment) Name() string { return "消息面分析师" }

// Analyze 分析公告情绪；无公告时本地返回中性，不消耗 LLM 调用。
func (s Sentiment) Analyze(ctx context.Context, c *Context) Result {
	if len(c.Announcements) == 0 {
		return Result{
			Agent:      s.Name(),
			Signal:     SignalNeutral,
			Confidence: 50,
			Reasoning:  "近期无公告数据，消息面按中性处理。",
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "请从消息面分析 %s(%s)，以下为最近公告 (日期 标题):\n\n", c.Name, c.Code)
	for _, a := range c.Announcements {
		fmt.Fprintf(&b, "- %s %s\n", a.Date, a.Title)
	}
	b.WriteString("\n请判断这些公告整体释放的情绪倾向 (利好/利空/中性) 并给出信号。仅依据标题，不要臆测公告内容。")
	return analyzeWithLLM(ctx, c, s.Name(), b.String())
}
