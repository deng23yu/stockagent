package agent

import (
	"context"
	"fmt"
)

// Fundamental 基本面分析师: 基于估值快照给出信号。
type Fundamental struct{}

// Name 返回分析师名称。
func (Fundamental) Name() string { return "基本面分析师" }

// Analyze 分析估值水平并输出信号。
func (f Fundamental) Analyze(ctx context.Context, c *Context) Result {
	s := c.Snapshot
	peNote := fmt.Sprintf("%.2f", s.PE)
	if s.PE <= 0 {
		peNote = "亏损或数据缺失"
	}
	pbNote := fmt.Sprintf("%.2f", s.PB)
	if s.PB <= 0 {
		pbNote = "数据缺失"
	}
	prompt := fmt.Sprintf(`请从基本面估值角度分析 %s(%s)，基于以下快照数据:

最新价: %.2f 元, 今日涨跌幅: %.2f%%
动态市盈率 PE: %s
市净率 PB: %s
总市值: %s, 流通市值: %s
换手率: %.2f%%

请结合 A 股同行业一般估值水平，判断当前估值的高低估程度并给出信号。数据为单日快照，深度财务分析不在本次范围内。`,
		c.Name, c.Code, s.Price, s.ChangePct, peNote, pbNote,
		FormatYi(s.TotalMarketCap), FormatYi(s.FloatMarketCap), s.TurnoverPct)
	return analyzeWithLLM(ctx, c, f.Name(), prompt)
}
