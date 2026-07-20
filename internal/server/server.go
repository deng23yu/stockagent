// Package server 提供 stockagent 的 HTTP API 服务。
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/eastmoney"
	"github.com/deng23yu/stockagent/internal/pipeline"
	"github.com/deng23yu/stockagent/internal/report"
	"github.com/deng23yu/stockagent/internal/tencent"
	"github.com/deng23yu/stockagent/internal/track"
)

const (
	// maxConcurrent 限制同时进行中的分析数 (单次分析包含多次 LLM 调用)。
	maxConcurrent = 4
	// maxCacheEntries 限制分析结果内存缓存条数，满时淘汰最早创建的一条。
	maxCacheEntries = 200
)

// Options 是 New 的配置项。
type Options struct {
	CacheTTL time.Duration  // 分析结果缓存时长 (<=0 默认 15min)
	Tracker  *track.Tracker // 访客记录器，nil 表示不记录；非 nil 时随 Close 一并关闭
	// AdminToken 非空时，访客查询接口 (/api/v1/visits*) 需携带
	// "Authorization: Bearer <token>" 或 "?token=<token>"。
	AdminToken string
	// TrustProxy 为 true 时才从 X-Forwarded-For / X-Real-IP 提取客户端 IP
	// (服务位于反向代理之后时开启；直连公网必须关闭，否则访客可伪造 IP)。
	TrustProxy bool
}

// Server 是 HTTP API 服务。
type Server struct {
	cfg *config.Config
	ttl time.Duration

	sem chan struct{} // 并发分析信号量

	mu    sync.Mutex
	cache map[string]cacheEntry

	tr         *track.Tracker
	em         *eastmoney.Client
	adminToken string
	trustProxy bool

	mktMu   sync.Mutex // 指数行情 60s 缓存
	mktBody []byte
	mktAt   time.Time
}

type cacheEntry struct {
	body      []byte
	createdAt time.Time
	expiresAt time.Time
}

// New 创建 Server。
func New(cfg *config.Config, opts Options) (*Server, error) {
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = 15 * time.Minute
	}
	return &Server{
		cfg:        cfg,
		ttl:        opts.CacheTTL,
		sem:        make(chan struct{}, maxConcurrent),
		cache:      make(map[string]cacheEntry),
		tr:         opts.Tracker,
		em:         eastmoney.New(nil),
		adminToken: opts.AdminToken,
		trustProxy: opts.TrustProxy,
	}, nil
}

// Close 释放服务持有的资源 (访客记录库)。
func (s *Server) Close() error {
	if s.tr != nil {
		return s.tr.Close()
	}
	return nil
}

// Handler 返回路由 (GET /healthz, GET /api/v1/*)，带访客记录与 CORS 中间件。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/v1/analyze", s.handleAnalyze)
	mux.HandleFunc("GET /api/v1/compare", s.handleCompare)
	mux.HandleFunc("GET /api/v1/market", s.handleMarket)
	mux.HandleFunc("GET /api/v1/hot-searches", s.handleHotSearches)
	mux.HandleFunc("GET /api/v1/access-log", s.handleAccessLog)
	mux.HandleFunc("GET /api/v1/visits", s.requireAdmin(s.handleVisits))
	mux.HandleFunc("GET /api/v1/visits/stats", s.requireAdmin(s.handleVisitsStats))
	mux.Handle("GET /", staticHandler()) // Web UI (SPA), API 路由优先匹配
	var h http.Handler = mux
	if s.tr != nil {
		h = s.tracking(h)
	}
	return cors(h)
}

// requireAdmin 包装管理接口: 设置了 adminToken 时校验 Bearer token / ?token=。
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminToken != "" && !s.validAdminToken(r) {
			writeError(w, http.StatusUnauthorized, "未授权: 需要 admin token")
			return
		}
		next(w, r)
	}
}

func (s *Server) validAdminToken(r *http.Request) bool {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tok == "" {
		tok = r.URL.Query().Get("token")
	}
	return subtle.ConstantTimeCompare([]byte(tok), []byte(s.adminToken)) == 1
}

// tracking 中间件: 记录每次访客请求 (IP/路径/搜索内容/状态码/耗时) 到访客库。
// 记录为异步提交，不增加请求延迟。
func (s *Server) tracking(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !trackable(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		rawQuery := r.URL.RawQuery
		if r.URL.Query().Has("token") { // 管理 token 不落库
			q := r.URL.Query()
			q.Del("token")
			rawQuery = q.Encode()
		}
		// 对比请求展开为每代码一条 (每只都真实跑了一轮分析，热门榜按代码计数);
		// 普通请求记一条，code 取自 query 参数。
		codes := []string{""}
		if r.URL.Path == "/api/v1/compare" {
			if cs := parseCodes(r.URL.Query().Get("codes")); len(cs) > 0 {
				codes = cs
			}
		} else if c := r.URL.Query().Get("code"); c != "" {
			codes[0] = c
		}
		for _, c := range codes {
			s.tr.Record(track.Visit{
				Time:      start,
				IP:        s.clientIP(r),
				Method:    r.Method,
				Path:      r.URL.Path,
				Query:     rawQuery,
				Code:      c,
				Source:    r.URL.Query().Get("source"),
				CacheHit:  sw.Header().Get("X-Cache") == "hit",
				Status:    sw.status,
				LatencyMs: time.Since(start).Milliseconds(),
				UserAgent: r.UserAgent(),
			})
		}
	})
}

// trackable 判定路径是否值得记录: 排除健康检查与静态资源 (避免刷屏)。
func trackable(p string) bool {
	if p == "/healthz" || strings.HasPrefix(p, "/assets/") {
		return false
	}
	return path.Ext(p) == "" // 带扩展名的 .js/.css/.png/ico 等文件不记录
}

// statusWriter 捕获响应状态码 (默认 200)。
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleAnalyze 处理 GET /api/v1/analyze?code=600519&source=ths。
func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "缺少必填参数 code (如 ?code=600519)")
		return
	}
	body, hit, status, err := s.analyzeStock(r.Context(), code, r.URL.Query().Get("source"))
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if hit {
		w.Header().Set("X-Cache", "hit")
	}
	writeRawJSON(w, status, body)
}

// analyzeStock 执行单只分析 (含结果缓存与并发上限)，analyze 与 compare 共用。
// 返回报告 JSON、是否缓存命中、建议的 HTTP 状态码与错误。
func (s *Server) analyzeStock(ctx context.Context, code, source string) ([]byte, bool, int, error) {
	key := source + ":" + code
	if body, ok := s.getCache(key); ok {
		return body, true, http.StatusOK, nil
	}

	// 超出并发上限直接拒绝，避免 LLM 侧排队失控
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	default:
		return nil, false, http.StatusTooManyRequests, errors.New("分析服务忙，请稍后重试")
	}

	data, err := pipeline.Run(ctx, s.cfg, code, pipeline.Options{Source: source},
		func(format string, args ...any) {
			log.Printf("[analyze %s] "+format, append([]any{code}, args...)...)
		})
	if err != nil {
		var ie *pipeline.InputError
		if errors.As(err, &ie) {
			return nil, false, http.StatusBadRequest, ie
		}
		return nil, false, http.StatusBadGateway, errors.New("分析失败: " + err.Error())
	}

	body, err := report.MarshalJSON(data)
	if err != nil {
		return nil, false, http.StatusInternalServerError, err
	}
	s.setCache(key, body)
	return body, false, http.StatusOK, nil
}

// compareItem 是多股对比中单只股票的结果 (成功含报告，失败含错误)。
type compareItem struct {
	Code   string          `json:"code"`
	OK     bool            `json:"ok"`
	Report json.RawMessage `json:"report,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// handleCompare 处理 GET /api/v1/compare?codes=600519,600737&source= (2~4 只)。
// 各代码并行分析 (共享 analyze 缓存与并发上限)，单只失败不影响整体。
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	codes := parseCodes(r.URL.Query().Get("codes"))
	if len(codes) < 2 || len(codes) > 4 {
		writeError(w, http.StatusBadRequest, "请输入 2~4 个股票代码 (逗号分隔，如 ?codes=600519,000001)")
		return
	}
	for _, c := range codes {
		if !isStockCode(c) {
			writeError(w, http.StatusBadRequest, "股票代码应为 6 位数字: "+c)
			return
		}
	}
	source := r.URL.Query().Get("source")

	items := make([]compareItem, len(codes))
	var wg sync.WaitGroup
	for i, c := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _, _, err := s.analyzeStock(r.Context(), c, source)
			if err != nil {
				items[i] = compareItem{Code: c, Error: err.Error()}
				return
			}
			items[i] = compareItem{Code: c, OK: true, Report: body}
		}()
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// parseCodes 解析逗号分隔的代码列表 (去空白、去重，保持顺序)。
func parseCodes(raw string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, c := range strings.Split(raw, ",") {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func isStockCode(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// handleMarket 处理 GET /api/v1/market，返回主要指数行情 (内存缓存 60s)。
func (s *Server) handleMarket(w http.ResponseWriter, r *http.Request) {
	s.mktMu.Lock()
	if len(s.mktBody) > 0 && time.Since(s.mktAt) < time.Minute {
		body := s.mktBody
		s.mktMu.Unlock()
		writeRawJSON(w, http.StatusOK, body)
		return
	}
	s.mktMu.Unlock()

	quotes, err := s.indexQuotes(r.Context())
	if err != nil {
		// 上游故障时降级返回陈旧缓存
		s.mktMu.Lock()
		defer s.mktMu.Unlock()
		if len(s.mktBody) > 0 {
			writeRawJSON(w, http.StatusOK, s.mktBody)
			return
		}
		writeError(w, http.StatusBadGateway, "指数行情拉取失败: "+err.Error())
		return
	}
	body, err := json.Marshal(map[string]any{
		"updated_at": time.Now().Format(time.RFC3339),
		"indices":    quotes,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.mktMu.Lock()
	s.mktBody, s.mktAt = body, time.Now()
	s.mktMu.Unlock()
	writeRawJSON(w, http.StatusOK, body)
}

// indexQuotes 并行拉取各指数: 报价 (东财) + 近 30 日收盘价 (腾讯, best-effort)。
// 单个指数失败则跳过，全部失败才返回错误。
func (s *Server) indexQuotes(ctx context.Context) ([]eastmoney.IndexQuote, error) {
	ids := eastmoney.DefaultIndexIDs
	results := make([]eastmoney.IndexQuote, len(ids))
	g, gctx := errgroup.WithContext(ctx)
	for i, id := range ids {
		g.Go(func() error {
			qs, err := s.em.IndexQuotes(gctx, []string{id})
			if err != nil {
				return err
			}
			if closes, err := tencent.DailyCloses(gctx, indexSymbol(id), 30); err == nil {
				qs[0].Closes = closes
			}
			results[i] = qs[0]
			return nil
		})
	}
	var lastErr error
	if err := g.Wait(); err != nil {
		lastErr = err
	}
	out := make([]eastmoney.IndexQuote, 0, len(ids))
	for _, q := range results {
		if q.Name != "" {
			out = append(out, q)
		}
	}
	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}

// indexSymbol 将东财 secid (1.000001/0.399001) 映射为腾讯代码 (sh000001/sz399001)。
func indexSymbol(secid string) string {
	if strings.HasPrefix(secid, "1.") {
		return "sh" + secid[2:]
	}
	return "sz" + secid[2:]
}

// handleHotSearches 处理 GET /api/v1/hot-searches?days=7，返回热门搜索代码 (公开)。
func (s *Server) handleHotSearches(w http.ResponseWriter, r *http.Request) {
	if s.tr == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []track.NameCount{}})
		return
	}
	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x > 0 && x <= 90 {
			days = x
		}
	}
	items, err := s.tr.QueryHotCodes(days, 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AccessRecord 是 /api/v1/access-log 的返回结构 (自 SQLite 的 analyze 记录映射，
// JSON 形状与旧版 JSONL 访问日志保持兼容)。
type AccessRecord struct {
	Time      time.Time `json:"time"`
	IP        string    `json:"ip"`
	Code      string    `json:"code"`
	Source    string    `json:"source"`
	CacheHit  bool      `json:"cache_hit"`
	Status    int       `json:"status"`
	LatencyMs int64     `json:"latency_ms"`
	UserAgent string    `json:"user_agent,omitempty"`
}

// handleAccessLog 处理 GET /api/v1/access-log?limit=50，
// 返回最近的 analyze 访问记录 (新的在前，数据来自访客库)。
func (s *Server) handleAccessLog(w http.ResponseWriter, r *http.Request) {
	if s.tr == nil {
		writeError(w, http.StatusServiceUnavailable, "访客记录未启用 (serve --db 置空)")
		return
	}
	n := parseLimit(r, 50)
	visits, err := s.tr.QueryVisits(n, track.Filter{Path: "/api/v1/analyze"})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	records := make([]AccessRecord, 0, len(visits))
	for _, v := range visits {
		records = append(records, AccessRecord{
			Time: v.Time, IP: v.IP, Code: v.Code, Source: v.Source,
			CacheHit: v.CacheHit, Status: v.Status, LatencyMs: v.LatencyMs, UserAgent: v.UserAgent,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

// handleVisits 处理 GET /api/v1/visits?limit=50&ip=&code=，从访客库查询记录 (新的在前)。
func (s *Server) handleVisits(w http.ResponseWriter, r *http.Request) {
	if s.tr == nil {
		writeError(w, http.StatusServiceUnavailable, "访客记录未启用 (serve --db 置空)")
		return
	}
	q := r.URL.Query()
	visits, err := s.tr.QueryVisits(parseLimit(r, 50), track.Filter{IP: q.Get("ip"), Code: q.Get("code")})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": visits})
}

// handleVisitsStats 处理 GET /api/v1/visits/stats，返回访客聚合统计。
func (s *Server) handleVisitsStats(w http.ResponseWriter, _ *http.Request) {
	if s.tr == nil {
		writeError(w, http.StatusServiceUnavailable, "访客记录未启用 (serve --db 置空)")
		return
	}
	stats, err := s.tr.QueryStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func parseLimit(r *http.Request, def int) int {
	n := def
	if v := r.URL.Query().Get("limit"); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x > 0 {
			n = min(x, 500)
		}
	}
	return n
}

// clientIP 提取客户端 IP。trustProxy 为 true (反向代理场景) 时优先
// X-Forwarded-For / X-Real-IP，否则一律取 RemoteAddr (防止伪造)。
func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i > 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
		if xri := r.Header.Get("X-Real-Ip"); xri != "" {
			return xri
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) getCache(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.cache[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.body, true
}

func (s *Server) setCache(key string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cache) >= maxCacheEntries {
		var oldestKey string
		var oldest time.Time
		for k, e := range s.cache {
			if oldestKey == "" || e.createdAt.Before(oldest) {
				oldestKey, oldest = k, e.createdAt
			}
		}
		delete(s.cache, oldestKey)
	}
	now := time.Now()
	s.cache[key] = cacheEntry{body: body, createdAt: now, expiresAt: now.Add(s.ttl)}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeRawJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ListenAndServe 启动服务并在 ctx 取消时优雅关闭。
func ListenAndServe(ctx context.Context, addr string, s *Server) error {
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      5 * time.Minute, // 单次分析可能耗时数十秒
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
