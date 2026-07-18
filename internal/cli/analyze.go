package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/pipeline"
	"github.com/deng23yu/stockagent/internal/report"
)

type analyzeOptions struct {
	days     int
	annCount int
	format   string
	output   string
	source   string
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
  stockagent analyze 300750 --source ths
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
	f.StringVar(&opts.source, "source", "eastmoney", "行情数据源: eastmoney (东方财富) | ths (同花顺)")
	f.StringVar(&opts.model, "model", "", "覆盖配置中的 LLM 模型")
	f.StringVar(&opts.baseURL, "base-url", "", "覆盖配置中的 LLM Base URL")
	f.StringVar(&opts.apiKey, "api-key", "", "覆盖配置中的 LLM API Key (也可用环境变量 STOCKAGENT_API_KEY)")
	return cmd
}

func runAnalyze(cmd *cobra.Command, code string, opts *analyzeOptions) error {
	cfg, err := config.Load(cfgFile, config.Overrides{
		BaseURL: opts.baseURL,
		APIKey:  opts.apiKey,
		Model:   opts.model,
	})
	if err != nil {
		return err
	}

	data, err := pipeline.Run(cmd.Context(), cfg, code, pipeline.Options{
		Days:     opts.days,
		AnnCount: opts.annCount,
		Source:   opts.source,
	}, func(format string, args ...any) {
		fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
	})
	if err != nil {
		return err
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
		fmt.Fprintf(cmd.ErrOrStderr(), "==> 报告已写入 %s\n", opts.output)
	}
	return nil
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
