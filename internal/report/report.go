// Package report 渲染投研报告 (终端 / Markdown / JSON)。
package report

import (
	"fmt"
	"time"

	"github.com/deng23yu/stockagent/internal/agent"
	"github.com/deng23yu/stockagent/internal/eastmoney"
)

// Data 是渲染报告所需的全部内容。
type Data struct {
	Code        string
	Name        string
	GeneratedAt time.Time
	Snapshot    *eastmoney.Snapshot
	Results     []agent.Result
	Final       agent.FinalVerdict
	Model       string
	Elapsed     time.Duration
}

// Disclaimer 风险提示语，出现在所有格式的报告中。
const Disclaimer = "本报告由 AI 自动生成，仅供学习与技术研究，不构成任何投资建议。股市有风险，投资需谨慎。"

func peText(pe float64) string {
	if pe <= 0 {
		return "亏损"
	}
	return fmt.Sprintf("%.2f", pe)
}
