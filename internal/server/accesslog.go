package server

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// AccessRecord 是一次 analyze 调用的访问记录。
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

// accessLog 记录访问日志: 追加到 JSONL 文件持久化，同时保留内存尾部供实时查询。
type accessLog struct {
	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
	buf []AccessRecord
	cap int
}

// newAccessLog 打开访问日志。path 为空时不落盘 (仅内存缓冲)。
func newAccessLog(path string, cap int) (*accessLog, error) {
	if cap <= 0 {
		cap = 500
	}
	al := &accessLog{cap: cap}
	if path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, err
		}
		al.f = f
		al.enc = json.NewEncoder(f)
	}
	return al, nil
}

func (al *accessLog) add(r AccessRecord) {
	al.mu.Lock()
	defer al.mu.Unlock()
	if al.enc != nil {
		_ = al.enc.Encode(r)
	}
	al.buf = append(al.buf, r)
	if len(al.buf) > al.cap {
		al.buf = al.buf[len(al.buf)-al.cap:]
	}
}

// recent 返回最近 n 条记录，新的在前。
func (al *accessLog) recent(n int) []AccessRecord {
	al.mu.Lock()
	defer al.mu.Unlock()
	if n <= 0 || n > len(al.buf) {
		n = len(al.buf)
	}
	out := make([]AccessRecord, 0, n)
	for i := len(al.buf) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, al.buf[i])
	}
	return out
}

func (al *accessLog) close() error {
	if al.f != nil {
		return al.f.Close()
	}
	return nil
}
