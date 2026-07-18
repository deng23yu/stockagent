package agent

import (
	"context"
	"fmt"
)

// Risk 风控官: 从波动与回撤角度评估风险并给出立场。
type Risk struct{}

// Name 返回分析师名称。
func (Risk) Name() string { return "风控官" }

// Analyze 评估风险水平并输出立场信号。
func (r Risk) Analyze(ctx context.Context, c *Context) Result {
	ind := c.Indicators
	var fromHigh, fromLow float64
	if ind.High52W > 0 {
		fromHigh = (ind.Price/ind.High52W - 1) * 100
	}
	if ind.Low52W > 0 {
		fromLow = (ind.Price/ind.Low52W - 1) * 100
	}
	prompt := fmt.Sprintf(`请从风险控制角度评估 %s(%s)，基于以下数据:

最新价: %.2f
年化波动率: %.1f%%
近一年最大回撤: %.1f%%
当前价相对52周高点: %.1f%%, 相对52周低点: %+.1f%%
近5日涨跌幅: %.2f%%, 近20日涨跌幅: %.2f%%
换手率: %.2f%%

请评估当前位置买入的风险收益比，给出风控视角的立场信号。`,
		c.Name, c.Code, ind.Price, ind.AnnualizedVol, ind.MaxDrawdown,
		fromHigh, fromLow, ind.ChangePct5, ind.ChangePct20, c.Snapshot.TurnoverPct)
	return analyzeWithLLM(ctx, c, r.Name(), prompt)
}
