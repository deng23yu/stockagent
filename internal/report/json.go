package report

import (
	"encoding/json"
	"io"

	"stockagent/internal/agent"
	"stockagent/internal/eastmoney"
)

type jsonReport struct {
	Code        string              `json:"code"`
	Name        string              `json:"name"`
	GeneratedAt string              `json:"generated_at"`
	Model       string              `json:"model"`
	Snapshot    *eastmoney.Snapshot `json:"snapshot"`
	Results     []agent.Result      `json:"results"`
	Final       agent.FinalVerdict  `json:"final"`
	Disclaimer  string              `json:"disclaimer"`
}

// RenderJSON 渲染 JSON 报告。
func RenderJSON(w io.Writer, d *Data) error {
	jr := jsonReport{
		Code:        d.Code,
		Name:        d.Name,
		GeneratedAt: d.GeneratedAt.Format("2006-01-02T15:04:05-07:00"),
		Model:       d.Model,
		Snapshot:    d.Snapshot,
		Results:     d.Results,
		Final:       d.Final,
		Disclaimer:  Disclaimer,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}
