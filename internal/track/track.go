// Package track 记录访客访问日志: 捕获每次 HTTP 请求的 IP、归属地 (省/市)、
// 路径、搜索内容等，异步批量写入 SQLite，供 /api/v1/visits 查询与聚合统计。
package track

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "modernc.org/sqlite" // database/sql 驱动 "sqlite" (纯 Go，无需 CGO)
)

const (
	// queueCap 为待写入记录的缓冲容量。打满时新记录直接丢弃，
	// 保证访客请求永远不被日志写入阻塞。
	queueCap = 256
	// batchSize / flushInterval 控制批量写入: 攒满 batchSize 条
	// 或距上一条到达超过 flushInterval 即单事务提交。
	batchSize     = 50
	flushInterval = 100 * time.Millisecond
	// cleanInterval 为过期记录清理周期。
	cleanInterval = 24 * time.Hour
)

// Visit 是一次 HTTP 请求的访客记录。
type Visit struct {
	ID        int64     `json:"id"`
	Time      time.Time `json:"time"`
	IP        string    `json:"ip"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Query     string    `json:"query,omitempty"` // 原始 query string (搜索内容)
	Code      string    `json:"code,omitempty"`  // 解析出的股票代码，便于检索
	Source    string    `json:"source,omitempty"`
	CacheHit  bool      `json:"cache_hit"`
	Status    int       `json:"status"`
	LatencyMs int64     `json:"latency_ms"`
	UserAgent string    `json:"user_agent,omitempty"`
	Country   string    `json:"country,omitempty"`
	Province  string    `json:"province,omitempty"`
	City      string    `json:"city,omitempty"`
}

// Filter 为 QueryVisits 的过滤条件，空字段表示不过滤。
type Filter struct {
	IP   string
	Code string
	Path string
}

// NameCount 是聚合统计的通用条目。
type NameCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// Stats 是访客聚合统计。
type Stats struct {
	TodayPV   int64       `json:"today_pv"`
	TodayUV   int64       `json:"today_uv"`
	TotalPV   int64       `json:"total_pv"`
	TotalUV   int64       `json:"total_uv"`
	TopCities []NameCount `json:"top_cities"`
	TopCodes  []NameCount `json:"top_codes"`
}

// Tracker 异步落库访客记录。Record 非阻塞；后台单 writer goroutine
// 依次做归属地解析 (带缓存) 和 SQLite 批量插入，天然规避写竞争。
type Tracker struct {
	db            *sql.DB
	geo           *geoResolver // nil 表示未加载 xdb: 公网 IP 归属地留空，内网 IP 仍标记 "内网"
	retentionDays int          // <=0 不清理历史记录

	mu     sync.RWMutex
	ch     chan Visit
	closed bool
	wg     sync.WaitGroup
}

// Open 打开访客记录库。dbPath 为空返回 (nil, nil) 表示禁用；
// ipdbPath 为 ip2region xdb 文件路径，文件不存在时降级为不解析公网归属地；
// retentionDays > 0 时自动清理更早的记录 (启动一次 + 每 24h)。
func Open(dbPath, ipdbPath string, retentionDays int) (*Tracker, error) {
	if dbPath == "" {
		return nil, nil
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // 单 writer，避免 SQLITE_BUSY
	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	var geo *geoResolver
	if ipdbPath != "" {
		if g, err := newGeoResolver(ipdbPath); err != nil {
			log.Printf("[track] ip2region 库 %s 不可用 (%v)，公网 IP 归属地将留空", ipdbPath, err)
		} else {
			geo = g
		}
	}

	t := &Tracker{db: db, geo: geo, retentionDays: retentionDays, ch: make(chan Visit, queueCap)}
	t.cleanOld() // 启动清理同步执行，保证 Open 返回时过期数据已删除
	t.wg.Add(1)
	go t.worker()
	return t, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS visits (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		time       TEXT NOT NULL,
		ip         TEXT NOT NULL,
		method     TEXT NOT NULL,
		path       TEXT NOT NULL,
		raw_query  TEXT NOT NULL DEFAULT '',
		code       TEXT NOT NULL DEFAULT '',
		source     TEXT NOT NULL DEFAULT '',
		cache_hit  INTEGER NOT NULL DEFAULT 0,
		status     INTEGER NOT NULL,
		latency_ms INTEGER NOT NULL,
		user_agent TEXT NOT NULL DEFAULT '',
		country    TEXT NOT NULL DEFAULT '',
		province   TEXT NOT NULL DEFAULT '',
		city       TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return err
	}
	// 老库迁移: 缺列则补 (PRAGMA table_info 检测)
	for _, col := range []string{"source", "cache_hit"} {
		ok, err := hasColumn(db, "visits", col)
		if err != nil {
			return err
		}
		if !ok {
			def := "TEXT NOT NULL DEFAULT ''"
			if col == "cache_hit" {
				def = "INTEGER NOT NULL DEFAULT 0"
			}
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE visits ADD COLUMN %s %s", col, def)); err != nil {
				return fmt.Errorf("迁移 visits 表加列 %s: %w", col, err)
			}
		}
	}
	for _, idx := range []string{
		`CREATE INDEX IF NOT EXISTS idx_visits_time ON visits(time)`,
		`CREATE INDEX IF NOT EXISTS idx_visits_ip ON visits(ip)`,
		`CREATE INDEX IF NOT EXISTS idx_visits_path ON visits(path)`,
	} {
		if _, err := db.Exec(idx); err != nil {
			return err
		}
	}
	// AI 信号战绩 (埋点积累，供后续战绩榜)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS signals (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		time       TEXT NOT NULL,
		code       TEXT NOT NULL,
		name       TEXT NOT NULL DEFAULT '',
		signal     TEXT NOT NULL,
		confidence INTEGER NOT NULL DEFAULT 0,
		price      REAL NOT NULL DEFAULT 0
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_signals_code ON signals(code)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS daily_closes (
		code  TEXT NOT NULL,
		date  TEXT NOT NULL,
		close REAL NOT NULL,
		PRIMARY KEY (code, date)
	)`); err != nil {
		return err
	}
	// 主力资金流日历史 (API 只给当日，历史靠每日快照自积累)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS fund_flow (
		code        TEXT NOT NULL,
		date        TEXT NOT NULL,
		main        REAL NOT NULL DEFAULT 0,
		super_large REAL NOT NULL DEFAULT 0,
		large       REAL NOT NULL DEFAULT 0,
		medium      REAL NOT NULL DEFAULT 0,
		small       REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (code, date)
	)`); err != nil {
		return err
	}
	return nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// Record 提交一条记录异步落库。队列满或已关闭时丢弃并告警，绝不阻塞调用方。
func (t *Tracker) Record(v Visit) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return
	}
	select {
	case t.ch <- v:
	default:
		log.Printf("[track] 记录队列已满，丢弃一条: %s %s (%s)", v.Method, v.Path, v.IP)
	}
}

// worker 串行消费记录: 解析归属地 (按 IP 缓存) 后批量写入 SQLite；
// 同时承担周期性的过期清理。
func (t *Tracker) worker() {
	defer t.wg.Done()
	cache := make(map[string][3]string)
	batch := make([]Visit, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := t.insertBatch(batch); err != nil {
			log.Printf("[track] 写入访客记录失败: %v", err)
		}
		batch = batch[:0]
	}

	flushTicker := time.NewTicker(flushInterval)
	cleanTicker := time.NewTicker(cleanInterval)
	defer flushTicker.Stop()
	defer cleanTicker.Stop()

	for {
		select {
		case v, ok := <-t.ch:
			if !ok {
				flush()
				return
			}
			geo := resolveGeo(cache, t.geo, v.IP)
			v.Country, v.Province, v.City = geo[0], geo[1], geo[2]
			batch = append(batch, v)
			if len(batch) >= batchSize {
				flush()
			}
		case <-flushTicker.C:
			flush()
		case <-cleanTicker.C:
			t.cleanOld()
		}
	}
}

func (t *Tracker) insertBatch(batch []Visit) error {
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Commit 后为 no-op
	stmt, err := tx.Prepare(
		`INSERT INTO visits (time, ip, method, path, raw_query, code, source, cache_hit, status, latency_ms, user_agent, country, province, city)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, v := range batch {
		if _, err := stmt.Exec(
			v.Time.Format(time.RFC3339), v.IP, v.Method, v.Path, v.Query, v.Code, v.Source,
			v.CacheHit, v.Status, v.LatencyMs, v.UserAgent, v.Country, v.Province, v.City); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// cleanOld 删除超出保留期的记录。
func (t *Tracker) cleanOld() {
	if t.retentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -t.retentionDays).Format(time.RFC3339)
	res, err := t.db.Exec(`DELETE FROM visits WHERE time < ?`, cutoff)
	if err != nil {
		log.Printf("[track] 清理过期记录失败: %v", err)
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("[track] 已清理 %d 条 %d 天前的访客记录", n, t.retentionDays)
	}
}

// QueryVisits 查询最近的访客记录，新的在前。limit 上限 500。
func (t *Tracker) QueryVisits(limit int, f Filter) ([]Visit, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	q := `SELECT id, time, ip, method, path, raw_query, code, source, cache_hit, status, latency_ms, user_agent, country, province, city
	      FROM visits`
	var args []any
	where := ""
	add := func(cond string, v string) {
		where += " AND " + cond
		args = append(args, v)
	}
	if f.IP != "" {
		add("ip = ?", f.IP)
	}
	if f.Code != "" {
		add("code = ?", f.Code)
	}
	if f.Path != "" {
		add("path = ?", f.Path)
	}
	if where != "" {
		q += " WHERE" + where[4:]
	}
	q += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := t.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Visit{}
	for rows.Next() {
		var v Visit
		var ts string
		if err := rows.Scan(&v.ID, &ts, &v.IP, &v.Method, &v.Path, &v.Query, &v.Code, &v.Source,
			&v.CacheHit, &v.Status, &v.LatencyMs, &v.UserAgent, &v.Country, &v.Province, &v.City); err != nil {
			return nil, err
		}
		if v.Time, err = time.Parse(time.RFC3339, ts); err != nil {
			return nil, fmt.Errorf("解析记录时间 %q: %w", ts, err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// QueryHotCodes 返回最近 days 天被搜索最多的股票代码 (公开榜单用)。
func (t *Tracker) QueryHotCodes(days, limit int) ([]NameCount, error) {
	if days <= 0 {
		days = 7
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
	rows, err := t.db.Query(
		`SELECT code, COUNT(*) AS c FROM visits
		 WHERE code != '' AND time >= ?
		 GROUP BY code ORDER BY c DESC, MAX(id) DESC LIMIT ?`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NameCount{}
	for rows.Next() {
		var nc NameCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, err
		}
		out = append(out, nc)
	}
	return out, rows.Err()
}

// Activity 是一条可公开的访客动态 (已脱敏: 仅城市级归属地与行为)。
type Activity struct {
	Time   time.Time `json:"time"`
	City   string    `json:"city"`
	Action string    `json:"action"` // analyze | compare
	Codes  []string  `json:"codes"`
}

// QueryRecentActivity 返回最近的访客动态 (新的在前)。
// 取 code 非空的记录；compare 同批展开的记录按 (ip, raw_query) 合并为一条多码动态；
// 内网记录 (多为站主自测) 不进入公开动态。
func (t *Tracker) QueryRecentActivity(limit int) ([]Activity, error) {
	if limit <= 0 || limit > 50 {
		limit = 15
	}
	rows, err := t.db.Query(
		`SELECT time, ip, path, raw_query, code, country, province FROM visits
		 WHERE code != '' AND country != '内网' ORDER BY id DESC LIMIT ?`, limit*4)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Activity{}
	mergedAt := make(map[string]int) // compare 合并键 -> out 下标
	for rows.Next() && len(out) < limit {
		var ts, ip, path, rawQuery, code, country, province string
		if err := rows.Scan(&ts, &ip, &path, &rawQuery, &code, &country, &province); err != nil {
			return nil, err
		}
		when, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		city := province
		if city == "" {
			city = country
		}
		if city == "" {
			city = "未知"
		}
		action := "analyze"
		mergeKey := ""
		if path == "/api/v1/compare" {
			action = "compare"
			mergeKey = ip + "|" + rawQuery
		}
		if mergeKey != "" {
			if i, ok := mergedAt[mergeKey]; ok {
				out[i].Codes = append(out[i].Codes, code)
				continue
			}
			mergedAt[mergeKey] = len(out)
		}
		out = append(out, Activity{Time: when, City: city, Action: action, Codes: []string{code}})
	}
	return out, rows.Err()
}

// Signal 是一次 AI 分析的信号事件 (战绩追踪埋点)。
type Signal struct {
	ID         int64     `json:"id"`
	Time       time.Time `json:"time"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Signal     string    `json:"signal"`
	Confidence int       `json:"confidence"`
	Price      float64   `json:"price"`
}

// RecordSignal 记录一次信号事件。database/sql 单连接串行化，可并发调用。
func (t *Tracker) RecordSignal(s Signal) error {
	_, err := t.db.Exec(
		`INSERT INTO signals (time, code, name, signal, confidence, price) VALUES (?,?,?,?,?,?)`,
		s.Time.Format(time.RFC3339), s.Code, s.Name, s.Signal, s.Confidence, s.Price)
	return err
}

// RecentSignals 返回最近的信号事件 (新的在前)，供战绩统计与测试。
func (t *Tracker) RecentSignals(limit int) ([]Signal, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := t.db.Query(
		`SELECT id, time, code, name, signal, confidence, price FROM signals ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Signal{}
	for rows.Next() {
		var s Signal
		var ts string
		if err := rows.Scan(&s.ID, &ts, &s.Code, &s.Name, &s.Signal, &s.Confidence, &s.Price); err != nil {
			return nil, err
		}
		if s.Time, err = time.Parse(time.RFC3339, ts); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RecentSignalCodes 返回近 days 天内有信号的代码 (去重)。
func (t *Tracker) RecentSignalCodes(days int) ([]string, error) {
	if days <= 0 {
		days = 45
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
	rows, err := t.db.Query(`SELECT DISTINCT code FROM signals WHERE time >= ?`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertDailyClose 写入某日收盘价 (已存在则忽略)。
func (t *Tracker) UpsertDailyClose(code, date string, close float64) error {
	_, err := t.db.Exec(`INSERT OR IGNORE INTO daily_closes (code, date, close) VALUES (?,?,?)`,
		code, date, close)
	return err
}

// FlowRecord 是某日资金流向五档 (元)。
type FlowRecord struct {
	Date       string
	Main       float64
	SuperLarge float64
	Large      float64
	Medium     float64
	Small      float64
}

// UpsertFundFlow 写入某日资金流向 (已存在则覆盖，允许当日盘中刷新)。
func (t *Tracker) UpsertFundFlow(code string, f FlowRecord) error {
	_, err := t.db.Exec(
		`INSERT INTO fund_flow (code, date, main, super_large, large, medium, small)
		 VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(code, date) DO UPDATE SET
		 main=excluded.main, super_large=excluded.super_large, large=excluded.large,
		 medium=excluded.medium, small=excluded.small`,
		code, f.Date, f.Main, f.SuperLarge, f.Large, f.Medium, f.Small)
	return err
}

// FundFlowHistory 返回某代码最近 days 天的资金流向 (按日期升序)。
func (t *Tracker) FundFlowHistory(code string, days int) ([]FlowRecord, error) {
	if days <= 0 {
		days = 5
	}
	rows, err := t.db.Query(
		`SELECT date, main, super_large, large, medium, small FROM fund_flow
		 WHERE code = ? ORDER BY date DESC LIMIT ?`, code, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FlowRecord{}
	for rows.Next() {
		var f FlowRecord
		if err := rows.Scan(&f.Date, &f.Main, &f.SuperLarge, &f.Large, &f.Medium, &f.Small); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	// DESC 取最近 N 条后翻转为升序
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// QueryStats 返回访客聚合统计 (PV/UV、Top 归属地与搜索代码)。
func (t *Tracker) QueryStats() (*Stats, error) {
	s := &Stats{}
	row := t.db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT ip) FROM visits`)
	if err := row.Scan(&s.TotalPV, &s.TotalUV); err != nil {
		return nil, err
	}

	// 今日: 本地零点起 (记录存 RFC3339 本地偏移，字符串可比)
	todayStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	row = t.db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT ip) FROM visits WHERE time >= ?`,
		todayStart.Format(time.RFC3339))
	if err := row.Scan(&s.TodayPV, &s.TodayUV); err != nil {
		return nil, err
	}

	cityRows, err := t.db.Query(
		`SELECT country, province, city, COUNT(*) AS c FROM visits
		 GROUP BY country, province, city ORDER BY c DESC LIMIT 11`)
	if err != nil {
		return nil, err
	}
	defer cityRows.Close()
	for cityRows.Next() {
		var co, p, ci string
		var nc NameCount
		if err := cityRows.Scan(&co, &p, &ci, &nc.Count); err != nil {
			return nil, err
		}
		nc.Name = joinNonEmpty(co, p, ci)
		if nc.Name == "" {
			nc.Name = "(未知)"
		}
		s.TopCities = append(s.TopCities, nc)
	}
	if err := cityRows.Err(); err != nil {
		return nil, err
	}

	codeRows, err := t.db.Query(
		`SELECT code, COUNT(*) AS c FROM visits WHERE code != ''
		 GROUP BY code ORDER BY c DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer codeRows.Close()
	for codeRows.Next() {
		var nc NameCount
		if err := codeRows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, err
		}
		s.TopCodes = append(s.TopCodes, nc)
	}
	return s, codeRows.Err()
}

func joinNonEmpty(parts ...string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += " "
		}
		out += p
	}
	return out
}

// Close 停止接收新记录，带超时地写完队列中剩余记录后关闭数据库。
func (t *Tracker) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	close(t.ch)
	t.mu.Unlock()

	done := make(chan struct{})
	go func() { t.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("[track] 关闭超时，剩余未写入的记录已丢弃")
	}
	if t.geo != nil {
		t.geo.close()
	}
	return t.db.Close()
}
