package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/server"
)

type serveOptions struct {
	host      string
	port      int
	cacheTTL  time.Duration
	accessLog string
	model     string
	baseURL   string
	apiKey    string
}

func newServeCmd() *cobra.Command {
	opts := &serveOptions{}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 HTTP API 服务",
		Example: `  stockagent serve --port 8080
  curl "http://127.0.0.1:8080/api/v1/analyze?code=600519&source=ths"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.host, "host", "127.0.0.1", "监听地址 (对外暴露用 0.0.0.0)")
	f.IntVar(&opts.port, "port", 8080, "监听端口")
	f.DurationVar(&opts.cacheTTL, "cache-ttl", 15*time.Minute, "分析结果缓存时长 (单次分析耗时数十秒)")
	f.StringVar(&opts.accessLog, "access-log", "access-log.jsonl", "访问日志 JSONL 文件路径 (置空则仅内存保留最近 500 条)")
	f.StringVar(&opts.model, "model", "", "覆盖配置中的 LLM 模型")
	f.StringVar(&opts.baseURL, "base-url", "", "覆盖配置中的 LLM Base URL")
	f.StringVar(&opts.apiKey, "api-key", "", "覆盖配置中的 LLM API Key (也可用环境变量 STOCKAGENT_API_KEY)")
	return cmd
}

func runServe(cmd *cobra.Command, opts *serveOptions) error {
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

	srv, err := server.New(cfg, opts.cacheTTL, opts.accessLog)
	if err != nil {
		return fmt.Errorf("访问日志: %w", err)
	}
	defer srv.Close()
	addr := opts.host + ":" + strconv.Itoa(opts.port)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(cmd.ErrOrStderr(), "stockagent serve 监听 http://%s (Ctrl+C 停止)\n", addr)
	fmt.Fprintln(cmd.ErrOrStderr(), "  GET /api/v1/analyze?code=600519&source=ths")
	return server.ListenAndServe(ctx, addr, srv)
}
