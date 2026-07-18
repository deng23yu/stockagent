package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/deng23yu/stockagent/internal/agent"
	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/eastmoney"
	"github.com/deng23yu/stockagent/internal/indicator"
	"github.com/deng23yu/stockagent/internal/llm"
	"github.com/deng23yu/stockagent/internal/report"
)

type analyzeOptions struct {
	days     int
	annCount int
	format   string
	output   string
	model    string
	baseURL  string
	apiKey   string
}

func newAnalyzeCmd() *cobra.Command {
	opts := &analyzeOptions{}
	cmd := &cobra.Command{
		Use:   "analyze <股票代码>",
		Short: "分析指定 A 股个股并生成 AI 投研报告",
		Example: `  stockagent analyze 600519
  stockagent analyze 300750 --format markdown -o report.md`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.IntVar(&opts.days, "days", 250, "拉取的日 K 线数量")
	f.IntVar(&opts.annCount, "ann", 20, "拉取的公告数量")
	f.StringVar(&opts.format, "format", "terminal", "输出格式: terminal | markdown | json")
	f.StringVarP(&opts.output, "output", "o", "", "写入文件 (默认输出到标准输出)")
	f.StringVar(&opts.model, "model", "", "覆盖配置中的 LLM 模型")
	f.StringVar(&opts.baseURL, "base-url", "", "覆盖配置中的 LLM Base URL")
	f.StringVar(&opts.apiKey, "api-key", "", "覆盖配置中的 LLM API Key (也可用环境变量 STOCKAGENT_API_KEY)")
	return cmd
}

func runAnalyze(cmd *cobra.Command, code string, opts *analyzeOptions) error {
	started := time.Now()
	ctx := cmd.Context()
	stderr := cmd.ErrOrStderr()

	secid, err := eastmoney.SecID(code)
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgFile, config.Overrides{
		BaseURL: opts.baseURL,
		APIKey:  opts.apiKey,
		Model:   opts.model,
	})
	if err != nil {
		return err
	}
	if cfg.LLM.APIKey == "" {
		return errors.New("未配置 LLM API Key\n请在 ./stockagent.yaml 或 ~/.stockagent.yaml 中填写 llm.api_key (参考 stockagent.yaml.example)，\n或设置环境变量 STOCKAGENT_API_KEY")
	}

	fmt.Fprintln(stderr, "==> 拉取行情与公告数据…")
	em := eastmoney.New(nil)
	var (
		klineData *eastmoney.KlineData
		snap      *eastmoney.Snapshot
		anns      []eastmoney.Announcement
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		d, err := em.Klines(gctx, secid, opts.days)
		if err != nil {
			return fmt.Errorf("K 线数据: %w", err)
		}
		klineData = d
		return nil
	})
	g.Go(func() error {
		s, err := em.Snapshot(gctx, secid)
		if err != nil {
			return fmt.Errorf("行情快照: %w", err)
		}
		snap = s
		return nil
	})
	g.Go(func() error {
		// 公告拉取失败不阻断主流程，消息面按无数据处理。
		a, err := em.Announcements(gctx, code, opts.annCount)
		if err == nil {
			anns = a
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return err
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
		Code:          klineData.Code,
		Name:          name,
		Snapshot:      snap,
		Indicators:    summary,
		Bars:          bars,
		Announcements: anns,
		LLM:           llm.New(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model),
	}

	fmt.Fprintln(stderr, "==> 4 位 AI 分析师并行分析中…")
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

	fmt.Fprintln(stderr, "==> 组合经理汇总中…")
	final := agent.PortfolioManager{}.Synthesize(ctx, actx, results)

	data := &report.Data{
		Code:        actx.Code,
		Name:        actx.Name,
		GeneratedAt: time.Now(),
		Snapshot:    snap,
		Results:     results,
		Final:       final,
		Model:       cfg.LLM.Model,
		Elapsed:     time.Since(started).Round(time.Second),
	}

	var w io.Writer = cmd.OutOrStdout()
	color := os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout)
	var f *os.File
	if opts.output != "" {
		f, err = os.Create(opts.output)
		if err != nil {
			return err
		}
		w = f
		color = false
	}

	switch opts.format {
	case "terminal":
		report.RenderTerminal(w, data, color)
	case "markdown":
		report.RenderMarkdown(w, data)
	case "json":
		err = report.RenderJSON(w, data)
	default:
		err = fmt.Errorf("未知输出格式 %q (可选: terminal | markdown | json)", opts.format)
	}
	if f != nil {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		return err
	}
	if opts.output != "" {
		fmt.Fprintf(stderr, "==> 报告已写入 %s\n", opts.output)
	}
	return nil
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
