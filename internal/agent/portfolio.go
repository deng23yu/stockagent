package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// FinalVerdict 是组合经理的综合结论。
type FinalVerdict struct {
	Signal     Signal   `json:"signal"`
	Confidence int      `json:"confidence"`
	Summary    string   `json:"summary"`
	KeyPoints  []string `json:"key_points"`
	Degraded   bool     `json:"degraded"` // true 表示 LLM 不可用时的本地聚合
}

// PortfolioManager 汇总各分析师信号为最终结论。
type PortfolioManager struct{}

const pmSystemPrompt = `你是一位严谨的 A 股投资组合经理。根据团队四位分析师的结论做最终决策，只输出 JSON，不要输出任何其他内容。JSON 格式:
{"signal":"bullish 或 bearish 或 neutral","confidence":0到100的整数,"summary":"200字以内的综合结论","key_points":["要点1","要点2","要点3"]}`

// Synthesize 汇总分析师结果为最终结论；LLM 失败时降级为本地加权聚合。
func (PortfolioManager) Synthesize(ctx context.Context, c *Context, results []Result) FinalVerdict {
	payload, _ := json.MarshalIndent(results, "", "  ")
	prompt := fmt.Sprintf(`以下是四位分析师对 %s(%s) 的分析结论 (JSON):

%s

请综合各方观点: 权衡各信号的方向与置信度，留意分析师之间的分歧，给出最终投资立场。`,
		c.Name, c.Code, payload)

	if content, err := c.LLM.Chat(ctx, pmSystemPrompt, prompt, true); err == nil {
		if v, perr := parseVerdict(content); perr == nil {
			return v
		}
	}
	return fallbackVerdict(results)
}

type verdictJSON struct {
	Signal     string   `json:"signal"`
	Confidence int      `json:"confidence"`
	Summary    string   `json:"summary"`
	KeyPoints  []string `json:"key_points"`
}

func parseVerdict(content string) (FinalVerdict, error) {
	var vj verdictJSON
	if err := json.Unmarshal([]byte(extractJSON(content)), &vj); err != nil {
		return FinalVerdict{}, err
	}
	if vj.Summary == "" {
		return FinalVerdict{}, fmt.Errorf("缺少 summary 字段")
	}
	return FinalVerdict{
		Signal:     normalizeSignal(vj.Signal),
		Confidence: clampConfidence(vj.Confidence),
		Summary:    vj.Summary,
		KeyPoints:  vj.KeyPoints,
	}, nil
}

// fallbackVerdict 在 LLM 不可用时用置信度加权打分做本地聚合。
func fallbackVerdict(results []Result) FinalVerdict {
	var score, weight float64
	for _, r := range results {
		if r.Err != "" || r.Confidence <= 0 {
			continue
		}
		w := float64(r.Confidence)
		switch r.Signal {
		case SignalBullish:
			score += w
		case SignalBearish:
			score -= w
		}
		weight += w
	}
	v := FinalVerdict{Signal: SignalNeutral, Confidence: 50, Degraded: true}
	if weight > 0 {
		avg := score / weight // 取值范围 [-1, 1]
		switch {
		case avg > 0.15:
			v.Signal = SignalBullish
		case avg < -0.15:
			v.Signal = SignalBearish
		}
		v.Confidence = int(math.Round(math.Min(math.Abs(avg)*100+40, 90)))
	}
	v.Summary = "LLM 组合经理不可用，本结论为各分析师信号的本地加权聚合，仅供参考。"
	for _, r := range results {
		if r.Err == "" {
			v.KeyPoints = append(v.KeyPoints,
				fmt.Sprintf("%s: %s (置信度 %d)", r.Agent, r.Signal.CN(), r.Confidence))
		}
	}
	return v
}
