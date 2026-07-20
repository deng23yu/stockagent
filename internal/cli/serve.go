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
	"github.com/deng23yu/stockagent/internal/track"
)

type serveOptions struct {
	host          string
	port          int
	cacheTTL      time.Duration
	db            string
	ipdb          string
	retentionDays int
	adminToken    string
	trustProxy    bool
	model         string
	baseURL       string
	apiKey        string
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
	f.StringVar(&opts.db, "db", "visits.db", "访客记录 SQLite 数据库路径 (置空则禁用访客记录)")
	f.StringVar(&opts.ipdb, "ipdb", "ip2region.xdb", "ip2region 数据文件路径 (用于解析 IP 省/市，不存在则归属地留空)")
	f.IntVar(&opts.retentionDays, "retention-days", 30, "访客记录保留天数 (0 表示永久保留)")
	f.StringVar(&opts.adminToken, "admin-token", "", "访客查询接口 (/api/v1/visits*) 的访问 token (置空则不鉴权)")
	f.BoolVar(&opts.trustProxy, "trust-proxy", false, "从 X-Forwarded-For/X-Real-IP 提取访客 IP (仅在反向代理之后开启)")
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

	tr, err := track.Open(opts.db, opts.ipdb, opts.retentionDays)
	if err != nil {
		return fmt.Errorf("访客数据库: %w", err)
	}
	if opts.adminToken == "" && opts.host != "127.0.0.1" && opts.host != "localhost" {
		fmt.Fprintln(cmd.ErrOrStderr(), "警告: 未设置 --admin-token，访客记录接口 (/api/v1/visits*) 将对所有人开放")
	}

	srv, err := server.New(cfg, server.Options{
		CacheTTL:   opts.cacheTTL,
		Tracker:    tr,
		AdminToken: opts.adminToken,
		TrustProxy: opts.trustProxy,
	})
	if err != nil {
		return err
	}
	defer srv.Close()
	addr := opts.host + ":" + strconv.Itoa(opts.port)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(cmd.ErrOrStderr(), "stockagent serve 监听 http://%s (Ctrl+C 停止)\n", addr)
	fmt.Fprintln(cmd.ErrOrStderr(), "  GET /api/v1/analyze?code=600519&source=ths")
	if tr != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "  GET /api/v1/visits?limit=50 (访客记录)")
	}
	return server.ListenAndServe(ctx, addr, srv)
}
