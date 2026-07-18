// Package server 提供 stockagent 的 HTTP API 服务。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/deng23yu/stockagent/internal/config"
	"github.com/deng23yu/stockagent/internal/pipeline"
	"github.com/deng23yu/stockagent/internal/report"
)

// maxConcurrent 限制同时进行中的分析数 (单次分析包含多次 LLM 调用)。
const maxConcurrent = 4

// Server 是 HTTP API 服务。
type Server struct {
	cfg *config.Config
	ttl time.Duration

	sem chan struct{} // 并发分析信号量

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	body      []byte
	expiresAt time.Time
}

// New 创建 Server。cacheTTL 为分析结果缓存时长 (单次分析耗时数十秒，缓存是必须的)。
func New(cfg *config.Config, cacheTTL time.Duration) *Server {
	if cacheTTL <= 0 {
		cacheTTL = 15 * time.Minute
	}
	return &Server{
		cfg:   cfg,
		ttl:   cacheTTL,
		sem:   make(chan struct{}, maxConcurrent),
		cache: make(map[string]cacheEntry),
	}
}

// Handler 返回路由 (GET /healthz, GET /api/v1/analyze)，带 CORS 中间件。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/v1/analyze", s.handleAnalyze)
	mux.Handle("GET /", staticHandler()) // Web UI (SPA), API 路由优先匹配
	return cors(mux)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
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

// handleAnalyze 处理 GET /api/v1/analyze?code=600519&source=ths
func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "缺少必填参数 code (如 ?code=600519)")
		return
	}
	source := r.URL.Query().Get("source")
	key := source + ":" + code

	if body, ok := s.getCache(key); ok {
		w.Header().Set("X-Cache", "hit")
		writeRawJSON(w, http.StatusOK, body)
		return
	}

	// 超出并发上限直接拒绝，避免 LLM 侧排队失控
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	default:
		writeError(w, http.StatusTooManyRequests, "分析服务忙，请稍后重试")
		return
	}

	data, err := pipeline.Run(r.Context(), s.cfg, code, pipeline.Options{Source: source},
		func(format string, args ...any) {
			log.Printf("[analyze %s] "+format, append([]any{code}, args...)...)
		})
	if err != nil {
		var ie *pipeline.InputError
		if errors.As(err, &ie) {
			writeError(w, http.StatusBadRequest, ie.Error())
			return
		}
		writeError(w, http.StatusBadGateway, "分析失败: "+err.Error())
		return
	}

	body, err := report.MarshalJSON(data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setCache(key, body)
	writeRawJSON(w, http.StatusOK, body)
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
	s.cache[key] = cacheEntry{body: body, expiresAt: time.Now().Add(s.ttl)}
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
