// Package agent 实现多位 AI 分析师智能体与组合经理。
package agent

import (
	"context"
	"fmt"

	"github.com/deng23yu/stockagent/internal/eastmoney"
	"github.com/deng23yu/stockagent/internal/indicator"
	"github.com/deng23yu/stockagent/internal/llm"
)

// Signal 是分析结论的方向。
type Signal string

const (
	SignalBullish Signal = "bullish"
	SignalBearish Signal = "bearish"
	SignalNeutral Signal = "neutral"
)

// CN 返回信号的中文名称。
func (s Signal) CN() string {
	switch s {
	case SignalBullish:
		return "看多"
	case SignalBearish:
		return "看空"
	default:
		return "中性"
	}
}

// Result 是一位分析师的输出。Err 非空表示分析失败。
type Result struct {
	Agent      string `json:"agent"`
	Signal     Signal `json:"signal"`
	Confidence int    `json:"confidence"`
	Reasoning  string `json:"reasoning"`
	Err        string `json:"err,omitempty"`
}

// Failed 构造一个分析失败的结果 (不阻断整体流程)。
func Failed(name string, err error) Result {
	return Result{
		Agent:      name,
		Signal:     SignalNeutral,
		Confidence: 0,
		Reasoning:  "分析失败: " + err.Error(),
		Err:        err.Error(),
	}
}

// Context 为智能体提供数据与 LLM 访问。
type Context struct {
	Code          string
	Name          string
	Snapshot      *eastmoney.Snapshot
	Indicators    indicator.Summary
	Bars          []eastmoney.Kline
	Announcements []eastmoney.Announcement
	LLM           *llm.Client
}

// Agent 是一位 AI 分析师。
type Agent interface {
	Name() string
	Analyze(ctx context.Context, c *Context) Result
}

// All 返回默认的四位分析师。
func All() []Agent {
	return []Agent{Technical{}, Fundamental{}, Sentiment{}, Risk{}}
}

const analystSystemPrompt = `你是一位严谨的 A 股证券分析师。根据用户提供的数据进行分析，只输出 JSON，不要输出任何其他内容。JSON 格式:
{"signal":"bullish 或 bearish 或 neutral","confidence":0到100的整数,"reasoning":"不超过150字的中文分析"}`

// analyzeWithLLM 是分析师的通用骨架: 调 LLM 并解析结构化结论。
func analyzeWithLLM(ctx context.Context, c *Context, name, userPrompt string) Result {
	content, err := c.LLM.Chat(ctx, analystSystemPrompt, userPrompt, true)
	if err != nil {
		return Failed(name, err)
	}
	r, err := parseResult(name, content)
	if err != nil {
		return Failed(name, fmt.Errorf("解析 LLM 输出失败 (%v), 原始输出: %s", err, truncate(content, 120)))
	}
	return r
}

// FormatYi 将金额 (元) 格式化为亿元/万亿元。
func FormatYi(v float64) string {
	switch {
	case v >= 1e12:
		return fmt.Sprintf("%.2f 万亿元", v/1e12)
	case v >= 1e8:
		return fmt.Sprintf("%.0f 亿元", v/1e8)
	default:
		return fmt.Sprintf("%.0f 元", v)
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
