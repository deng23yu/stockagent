package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deng23yu/stockagent/internal/agent"
	"github.com/deng23yu/stockagent/internal/eastmoney"
)

func sampleData() *Data {
	return &Data{
		Code:        "600519",
		Name:        "贵州茅台",
		GeneratedAt: time.Date(2026, 7, 18, 12, 0, 0, 0, time.Local),
		Snapshot: &eastmoney.Snapshot{
			Code: "600519", Name: "贵州茅台", Price: 1253, ChangePct: -0.48,
			PE: 14.37, PB: 6.64, TotalMarketCap: 1.566e12,
		},
		Results: []agent.Result{
			{Agent: "技术面分析师", Signal: agent.SignalBullish, Confidence: 72, Reasoning: "均线多头排列。"},
			{Agent: "消息面分析师", Signal: agent.SignalNeutral, Confidence: 50, Reasoning: "近期无公告数据。"},
		},
		Final: agent.FinalVerdict{
			Signal: agent.SignalBullish, Confidence: 65,
			Summary:   "技术面偏强，消息面平淡，综合谨慎看多。",
			KeyPoints: []string{"要点一", "要点二"},
		},
		Model:   "deepseek-chat",
		Elapsed: 12 * time.Second,
	}
}

func TestRenderTerminalNoColor(t *testing.T) {
	var buf bytes.Buffer
	RenderTerminal(&buf, sampleData(), false)
	out := buf.String()
	for _, want := range []string{"AI 投研报告", "贵州茅台 (600519)", "综合结论: 看多 (置信度 65)", "技术面分析师", "风险提示"} {
		if !strings.Contains(out, want) {
			t.Errorf("终端输出缺少 %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "\033[") {
		t.Error("color=false 时不应出现 ANSI 转义")
	}
}

func TestRenderMarkdown(t *testing.T) {
	var buf bytes.Buffer
	RenderMarkdown(&buf, sampleData())
	out := buf.String()
	for _, want := range []string{"# 贵州茅台 (600519) AI 投研报告", "| 智能体 | 信号 | 置信度 |", "## 综合结论: 看多 (置信度 65)", "1. 要点一", Disclaimer} {
		if !strings.Contains(out, want) {
			t.Errorf("Markdown 输出缺少 %q\n%s", want, out)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderJSON(&buf, sampleData()); err != nil {
		t.Fatal(err)
	}
	var v struct {
		Code       string `json:"code"`
		Disclaimer string `json:"disclaimer"`
		Final      struct {
			Signal string `json:"signal"`
		} `json:"final"`
	}
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatalf("JSON 输出不可解析: %v", err)
	}
	if v.Code != "600519" || v.Final.Signal != "bullish" || v.Disclaimer == "" {
		t.Errorf("JSON 内容异常: %+v", v)
	}
}
