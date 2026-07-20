// Package pipeline 串联完整分析流程: 数据拉取 → 指标计算 → 多智能体并行 → 组合经理汇总。
//
// 同一份流程同时服务于 CLI (analyze) 与 HTTP API (serve)。
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/deng23yu/stockagent/internal/agent"
	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/eastmoney"
	"github.com/deng23yu/stockagent/internal/indicator"
	"github.com/deng23yu/stockagent/internal/llm"
	"github.com/deng23yu/stockagent/internal/report"
	"github.com/deng23yu/stockagent/internal/ths"
)

// Options 控制一次分析的参数，零值取默认值。
type Options struct {
	Days     int    // 拉取的日 K 线数量, 默认 250
	AnnCount int    // 拉取的公告数量, 默认 20
	Source   string // 行情数据源: eastmoney (默认) | ths
}

func (o *Options) defaults() {
	if o.Days <= 0 {
		o.Days = 250
	}
	if o.AnnCount <= 0 {
		o.AnnCount = 20
	}
	if o.Source == "" {
		o.Source = "eastmoney"
	}
}

// InputError 表示调用方参数错误 (股票代码/数据源非法)，调用方据此返回 400。
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

func inputErrorf(format string, args ...any) *InputError {
	return &InputError{Msg: fmt.Sprintf(format, args...)}
}

// Run 执行完整分析流程并返回报告数据。logf 为 nil 时不输出进度日志。
func Run(ctx context.Context, cfg *config.Config, code string, opts Options, logf func(string, ...any)) (*report.Data, error) {
	started := time.Now()
	opts.defaults()
	log := func(format string, args ...any) {
		if logf != nil {
			logf(format, args...)
		}
	}

	if cfg.LLM.APIKey == "" {
		return nil, errors.New("未配置 LLM API Key\n请在 ./stockagent.yaml 或 ~/.stockagent.yaml 中填写 llm.api_key (参考 stockagent.yaml.example)，\n或设置环境变量 STOCKAGENT_API_KEY")
	}
	if err := validateCode(code); err != nil {
		return nil, err
	}

	em := eastmoney.New(nil)
	var src eastmoney.Source
	switch opts.Source {
	case "eastmoney":
		src = em
	case "ths":
		src = ths.New(nil)
	default:
		return nil, inputErrorf("未知数据源 %q (可选: eastmoney | ths)", opts.Source)
	}

	log("==> 拉取行情与公告数据…")
	var (
		klineData *eastmoney.KlineData
		snap      *eastmoney.Snapshot
		anns      []eastmoney.Announcement
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		d, err := src.KlinesByCode(gctx, code, opts.Days)
		if err != nil && opts.Source == "eastmoney" {
			// 东财 K 线域名可能对机房 IP 限流/封禁，回退同花顺 K 线兜底
			log("东财 K 线拉取失败 (%v)，回退同花顺 K 线…", err)
			d, err = ths.New(nil).KlinesByCode(gctx, code, opts.Days)
		}
		if err != nil {
			return fmt.Errorf("K 线数据: %w", err)
		}
		klineData = d
		return nil
	})
	g.Go(func() error {
		s, err := src.SnapshotByCode(gctx, code)
		if err != nil {
			return fmt.Errorf("行情快照: %w", err)
		}
		snap = s
		return nil
	})
	g.Go(func() error {
		// 公告始终用东方财富接口 (ths 无免费公告源，东财公告子域与行情子域限流独立);
		// 拉取失败不阻断主流程，消息面按无数据处理。
		a, err := em.Announcements(gctx, code, opts.AnnCount)
		if err == nil {
			anns = a
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	closes := make([]float64, len(klineData.Bars))
	highs := make([]float64, len(klineData.Bars))
	lows := make([]float64, len(klineData.Bars))
	for i, b := range klineData.Bars {
		closes[i], highs[i], lows[i] = b.Close, b.High, b.Low
	}
	summary := indicator.Summarize(closes, highs, lows)

	name := klineData.Name
	if name == "" {
		name = snap.Name
	}
	bars := klineData.Bars
	if len(bars) > 30 {
		bars = bars[len(bars)-30:]
	}
	actx := &agent.Context{
		Code:          code,
		Name:          name,
		Snapshot:      snap,
		Indicators:    summary,
		Bars:          bars,
		Announcements: anns,
		LLM:           llm.New(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.Temperature),
	}

	log("==> 4 位 AI 分析师并行分析中…")
	agents := agent.All()
	results := make([]agent.Result, len(agents))
	g2, g2ctx := errgroup.WithContext(ctx)
	for i, ag := range agents {
		g2.Go(func() error {
			results[i] = ag.Analyze(g2ctx, actx) // 失败体现在 Result.Err，不中断其他分析师
			return nil
		})
	}
	_ = g2.Wait()

	log("==> 组合经理汇总中…")
	final := agent.PortfolioManager{}.Synthesize(ctx, actx, results)

	return &report.Data{
		Code:        actx.Code,
		Name:        actx.Name,
		GeneratedAt: time.Now(),
		Snapshot:    snap,
		Results:     results,
		Final:       final,
		Model:       cfg.LLM.Model,
		Elapsed:     time.Since(started).Round(time.Second),
	}, nil
}

// validateCode 校验 6 位数字代码 (具体市场合法性由数据源各自把关)。
func validateCode(code string) error {
	if len(code) != 6 {
		return inputErrorf("股票代码应为 6 位数字，收到 %q", code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return inputErrorf("股票代码应为 6 位数字，收到 %q", code)
		}
	}
	return nil
}
