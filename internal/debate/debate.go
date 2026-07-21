// Package debate 多空辩论赛: 多方辩手与空方辩手基于同一份数据公开对辩，
// 两轮交锋后由裁判裁决胜负。每轮发言可见此前全部记录，串行调用 LLM。
package debate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/deng23yu/stockagent/internal/agent"
)

// Turn 是一轮发言。
type Turn struct {
	Role    string `json:"role"` // bull | bear
	Round   int    `json:"round"`
	Content string `json:"content"`
}

// Verdict 是裁判裁决。
type Verdict struct {
	Winner    string `json:"winner"` // bull | bear | draw
	BullScore int    `json:"bull_score"`
	BearScore int    `json:"bear_score"`
	Reasoning string `json:"reasoning"`
}

// Debate 是一场完整的多空辩论。
type Debate struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Turns       []Turn    `json:"turns"`
	Verdict     Verdict   `json:"verdict"`
	Model       string    `json:"model"`
	GeneratedAt time.Time `json:"generated_at"`
}

const (
	bullSystem = `你是 A 股多头辩手"老多"。你坚信这只股票有上涨逻辑，任务是说服观众。
要求: 观点鲜明、论据具体 (引用给出的数据)、语言泼辣带江湖气但不失专业，可以调侃对手。
每次发言不超过 150 字，直接说，不要自我介绍。`

	bearSystem = `你是 A 股空头辩手"老空"。你坚信这只股票风险大于机会，任务是说服观众。
要求: 观点鲜明、论据具体 (引用给出的数据)、语言冷静犀利、善于抓对方逻辑漏洞。
每次发言不超过 150 字，直接说，不要自我介绍。`

	judgeSystem = `你是一场 A 股多空辩论的裁判。阅读双方发言后裁决胜负，只输出 JSON，不要输出其他内容。
JSON 格式: {"winner":"bull 或 bear 或 draw","bull_score":0到100整数,"bear_score":0到100整数,"reasoning":"不超过100字的裁决理由"}
评分依据: 论据数据支撑度、逻辑严密性、对对方观点的回应质量。`
)

// Run 基于已准备的分析上下文执行一场辩论 (5 次串行 LLM 调用)。
func Run(ctx context.Context, actx *agent.Context) (*Debate, error) {
	brief := dataBrief(actx)
	d := &Debate{Code: actx.Code, Name: actx.Name, GeneratedAt: time.Now()}

	say := func(system, user string) (string, error) {
		out, err := actx.LLM.Chat(ctx, system, user, false)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}

	bullOpen, err := say(bullSystem, brief+"\n请你做开场立论。")
	if err != nil {
		return nil, fmt.Errorf("多方立论: %w", err)
	}
	d.Turns = append(d.Turns, Turn{Role: "bull", Round: 1, Content: bullOpen})

	bearOpen, err := say(bearSystem, brief+"\n多方辩手刚刚立论:\n"+bullOpen+"\n请你做开场立论 (可顺手回应对方)。")
	if err != nil {
		return nil, fmt.Errorf("空方立论: %w", err)
	}
	d.Turns = append(d.Turns, Turn{Role: "bear", Round: 1, Content: bearOpen})

	bullRe, err := say(bullSystem, brief+"\n空方辩手立论:\n"+bearOpen+"\n请反驳对方观点。")
	if err != nil {
		return nil, fmt.Errorf("多方反驳: %w", err)
	}
	d.Turns = append(d.Turns, Turn{Role: "bull", Round: 2, Content: bullRe})

	bearRe, err := say(bearSystem, brief+"\n多方辩手反驳:\n"+bullRe+"\n请针锋相对地回应。")
	if err != nil {
		return nil, fmt.Errorf("空方反驳: %w", err)
	}
	d.Turns = append(d.Turns, Turn{Role: "bear", Round: 2, Content: bearRe})

	var transcript strings.Builder
	for _, t := range d.Turns {
		who := "多方"
		if t.Role == "bear" {
			who = "空方"
		}
		fmt.Fprintf(&transcript, "【%s 第%d轮】%s\n", who, t.Round, t.Content)
	}
	raw, err := say(judgeSystem, brief+"\n辩论实录:\n"+transcript.String()+"\n请裁决。")
	if err != nil {
		return nil, fmt.Errorf("裁判裁决: %w", err)
	}
	d.Verdict = parseVerdict(raw)
	return d, nil
}

// parseVerdict 从 LLM 输出中解析裁决 JSON，失败时降级为平局。
func parseVerdict(raw string) Verdict {
	v := Verdict{Winner: "draw", BullScore: 50, BearScore: 50, Reasoning: "裁判未能给出明确裁决。"}
	i, j := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if i < 0 || j <= i {
		return v
	}
	var parsed struct {
		Winner    string `json:"winner"`
		BullScore int    `json:"bull_score"`
		BearScore int    `json:"bear_score"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(raw[i:j+1]), &parsed); err != nil {
		return v
	}
	switch parsed.Winner {
	case "bull", "bear", "draw":
		v.Winner = parsed.Winner
	}
	if parsed.BullScore > 0 {
		v.BullScore = parsed.BullScore
	}
	if parsed.BearScore > 0 {
		v.BearScore = parsed.BearScore
	}
	if parsed.Reasoning != "" {
		v.Reasoning = parsed.Reasoning
	}
	return v
}

// dataBrief 把分析上下文压缩成辩论双方共用的数据简报。
func dataBrief(c *agent.Context) string {
	var b strings.Builder
	s := c.Snapshot
	fmt.Fprintf(&b, "标的: %s(%s) 现价 %.2f 涨跌幅 %+.2f%%\n", c.Name, c.Code, s.Price, s.ChangePct)
	fmt.Fprintf(&b, "估值: PE %.2f PB %.2f 总市值 %.0f 亿\n", s.PE, s.PB, s.TotalMarketCap/1e8)
	i := c.Indicators
	fmt.Fprintf(&b, "趋势: 5日 %+.1f%% 20日 %+.1f%% 60日 %+.1f%% | MA5 %.2f MA20 %.2f MA60 %.2f\n",
		i.ChangePct5, i.ChangePct20, i.ChangePct60, i.MA5, i.MA20, i.MA60)
	fmt.Fprintf(&b, "指标: MACD DIF %.2f DEA %.2f | RSI14 %.1f | 年化波动 %.1f%% | 52周区间 %.2f~%.2f\n",
		i.DIF, i.DEA, i.RSI14, i.AnnualizedVol, i.Low52W, i.High52W)
	if len(c.Bars) > 0 {
		first, last := c.Bars[0], c.Bars[len(c.Bars)-1]
		fmt.Fprintf(&b, "近 %d 个交易日: %s %.2f → %s %.2f\n", len(c.Bars), first.Date, first.Close, last.Date, last.Close)
	}
	if len(c.Announcements) > 0 {
		b.WriteString("近期公告:")
		for k, a := range c.Announcements {
			if k >= 5 {
				break
			}
			fmt.Fprintf(&b, " [%s]%s", a.Date, a.Title)
		}
		b.WriteString("\n")
	}
	return b.String()
}
