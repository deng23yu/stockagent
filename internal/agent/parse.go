package agent

import (
	"encoding/json"
	"errors"
	"strings"
)

// extractJSON 从可能夹杂 Markdown 代码块或散文的输出中提取 JSON 对象。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return s
	}
	return s[i : j+1]
}

// normalizeSignal 兼容中英文的各种写法。
func normalizeSignal(s string) Signal {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(s, "bull"), strings.Contains(s, "看多"),
		s == "buy", strings.Contains(s, "买入"):
		return SignalBullish
	case strings.Contains(s, "bear"), strings.Contains(s, "看空"),
		s == "sell", strings.Contains(s, "卖出"):
		return SignalBearish
	default:
		return SignalNeutral
	}
}

func clampConfidence(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

type resultJSON struct {
	Signal     string `json:"signal"`
	Confidence int    `json:"confidence"`
	Reasoning  string `json:"reasoning"`
}

// parseResult 解析分析师的 JSON 输出。
func parseResult(name, content string) (Result, error) {
	var rj resultJSON
	if err := json.Unmarshal([]byte(extractJSON(content)), &rj); err != nil {
		return Result{}, err
	}
	if rj.Reasoning == "" {
		return Result{}, errors.New("缺少 reasoning 字段")
	}
	return Result{
		Agent:      name,
		Signal:     normalizeSignal(rj.Signal),
		Confidence: clampConfidence(rj.Confidence),
		Reasoning:  rj.Reasoning,
	}, nil
}
