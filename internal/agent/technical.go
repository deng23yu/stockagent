package agent

import (
	"context"
	"fmt"
)

// Technical 技术面分析师: 基于 K 线指标给出信号。
type Technical struct{}

// Name 返回分析师名称。
func (Technical) Name() string { return "技术面分析师" }

// Analyze 分析技术指标并输出信号。
func (t Technical) Analyze(ctx context.Context, c *Context) Result {
	ind := c.Indicators
	prompt := fmt.Sprintf(`请从技术面分析 %s(%s)，基于以下最近约一年的日线指标:

最新价: %.2f
涨跌幅: 近5日 %.2f%%, 近20日 %.2f%%, 近60日 %.2f%%, 近120日 %.2f%%
均线: MA5=%.2f, MA10=%.2f, MA20=%.2f, MA60=%.2f
MACD(12,26,9): DIF=%.3f, DEA=%.3f, BAR=%.3f
RSI14: %.1f
52周区间: %.2f ~ %.2f
年化波动率: %.1f%%, 区间最大回撤: %.1f%%

请综合趋势、动量与超买超卖状态给出技术面信号。`,
		c.Name, c.Code,
		ind.Price,
		ind.ChangePct5, ind.ChangePct20, ind.ChangePct60, ind.ChangePct120,
		ind.MA5, ind.MA10, ind.MA20, ind.MA60,
		ind.DIF, ind.DEA, ind.BAR,
		ind.RSI14,
		ind.Low52W, ind.High52W,
		ind.AnnualizedVol, ind.MaxDrawdown)
	return analyzeWithLLM(ctx, c, t.Name(), prompt)
}
