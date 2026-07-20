package track

import (
	"path/filepath"
	"testing"
	"time"
)

// waitRows 轮询等待异步 writer 落库 (Record 是异步批量写入)。
func waitRows(t *testing.T, tr *Tracker, want int) []Visit {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		recs, err := tr.QueryVisits(100, Filter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(recs) >= want {
			return recs
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待落库超时: 期望至少 %d 条", want)
	return nil
}

func TestTrackerRecordAndQuery(t *testing.T) {
	tr, err := Open(filepath.Join(t.TempDir(), "visits.db"), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()

	tr.Record(Visit{
		Time: time.Now(), IP: "192.168.1.8", Method: "GET", Path: "/",
		Status: 200, LatencyMs: 3, UserAgent: "curl/8",
	})
	tr.Record(Visit{
		Time: time.Now(), IP: "203.0.113.7", Method: "GET", Path: "/api/v1/analyze",
		Query: "code=600519&source=ths", Code: "600519", Source: "ths", Status: 200, LatencyMs: 1500,
	})

	recs := waitRows(t, tr, 2)
	// 新的在前
	if recs[0].Path != "/api/v1/analyze" || recs[1].Path != "/" {
		t.Fatalf("排序应为新的在前: %+v", recs)
	}
	analyze := recs[0]
	if analyze.Query != "code=600519&source=ths" || analyze.Code != "600519" ||
		analyze.Source != "ths" || analyze.Status != 200 {
		t.Errorf("analyze 记录内容异常: %+v", analyze)
	}
	// 无 xdb: 内网 IP 仍标记 "内网"，公网 IP 归属地留空
	if got := recs[1].Country; got != "内网" {
		t.Errorf("内网 IP Country = %q, want 内网", got)
	}
	if analyze.Country != "" || analyze.City != "" {
		t.Errorf("无 xdb 时公网归属地应为空: %+v", analyze)
	}

	// 过滤查询
	byIP, err := tr.QueryVisits(10, Filter{IP: "192.168.1.8"})
	if err != nil || len(byIP) != 1 || byIP[0].Path != "/" {
		t.Errorf("按 ip 过滤异常: %v %+v", err, byIP)
	}
	byCode, err := tr.QueryVisits(10, Filter{Code: "600519"})
	if err != nil || len(byCode) != 1 || byCode[0].Path != "/api/v1/analyze" {
		t.Errorf("按 code 过滤异常: %v %+v", err, byCode)
	}
	byPath, err := tr.QueryVisits(10, Filter{Path: "/api/v1/analyze"})
	if err != nil || len(byPath) != 1 {
		t.Errorf("按 path 过滤异常: %v %+v", err, byPath)
	}
}

func TestTrackerCloseDrains(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "visits.db")
	tr, err := Open(dbPath, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	tr.Record(Visit{Time: time.Now(), IP: "10.0.0.1", Method: "GET", Path: "/", Status: 200})
	if err := tr.Close(); err != nil { // 立即关闭应仍能写完队列中的记录
		t.Fatal(err)
	}
	tr.Record(Visit{Time: time.Now(), IP: "10.0.0.2", Method: "GET", Path: "/", Status: 200}) // 关闭后 Record 不应 panic
	if err := tr.Close(); err != nil {                                                        // 重复关闭安全
		t.Fatal(err)
	}

	// 重新打开同一文件验证持久化
	tr2, err := Open(dbPath, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Close()
	recs, err := tr2.QueryVisits(10, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].IP != "10.0.0.1" {
		t.Fatalf("Close 后应有 1 条持久化记录: %+v", recs)
	}
}

// TestRetention 验证过期记录清理: 启动时即删除超出保留天数的记录。
func TestRetention(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "visits.db")
	tr, err := Open(dbPath, "", 0) // 先不清理地写入
	if err != nil {
		t.Fatal(err)
	}
	tr.Record(Visit{Time: time.Now().AddDate(0, 0, -40), IP: "10.0.0.1", Method: "GET", Path: "/old", Status: 200})
	tr.Record(Visit{Time: time.Now(), IP: "10.0.0.1", Method: "GET", Path: "/new", Status: 200})
	waitRows(t, tr, 2)
	tr.Close()

	tr2, err := Open(dbPath, "", 30) // 保留 30 天，启动即清理
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Close()
	recs, err := tr2.QueryVisits(10, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Path != "/new" {
		t.Fatalf("40 天前的记录应被清理: %+v", recs)
	}
}

// TestQueryStats 验证聚合统计。
func TestQueryStats(t *testing.T) {
	tr, err := Open(filepath.Join(t.TempDir(), "visits.db"), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()

	tr.Record(Visit{Time: time.Now(), IP: "10.0.0.1", Method: "GET", Path: "/", Status: 200})
	tr.Record(Visit{Time: time.Now(), IP: "10.0.0.1", Method: "GET", Path: "/", Status: 200})
	tr.Record(Visit{Time: time.Now(), IP: "192.168.0.2", Method: "GET", Path: "/api/v1/analyze", Code: "600519", Status: 200})
	waitRows(t, tr, 3)

	s, err := tr.QueryStats()
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalPV != 3 || s.TotalUV != 2 {
		t.Errorf("累计 PV/UV = %d/%d, want 3/2", s.TotalPV, s.TotalUV)
	}
	if s.TodayPV != 3 || s.TodayUV != 2 {
		t.Errorf("今日 PV/UV = %d/%d, want 3/2", s.TodayPV, s.TodayUV)
	}
	if len(s.TopCodes) != 1 || s.TopCodes[0].Name != "600519" || s.TopCodes[0].Count != 1 {
		t.Errorf("TopCodes 异常: %+v", s.TopCodes)
	}
	// 两条 10.0.0.1 (内网) 应聚合为 count=2
	if len(s.TopCities) == 0 || s.TopCities[0].Name != "内网" || s.TopCities[0].Count != 3 {
		t.Errorf("TopCities 异常: %+v", s.TopCities)
	}
}

// TestSchemaMigration 验证老库 (缺 source/cache_hit 列) 自动迁移。
func TestSchemaMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "visits.db")
	// 手工建老版表
	tr, err := Open(dbPath, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tr.db.Exec(`ALTER TABLE visits RENAME TO visits_new`); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.db.Exec(`CREATE TABLE visits (
		id INTEGER PRIMARY KEY AUTOINCREMENT, time TEXT NOT NULL, ip TEXT NOT NULL,
		method TEXT NOT NULL, path TEXT NOT NULL, raw_query TEXT NOT NULL DEFAULT '',
		code TEXT NOT NULL DEFAULT '', status INTEGER NOT NULL, latency_ms INTEGER NOT NULL,
		user_agent TEXT NOT NULL DEFAULT '', country TEXT NOT NULL DEFAULT '',
		province TEXT NOT NULL DEFAULT '', city TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.db.Exec(`INSERT INTO visits (time, ip, method, path, status, latency_ms)
		VALUES ('2026-07-01T00:00:00+08:00', '1.2.3.4', 'GET', '/', 200, 1)`); err != nil {
		t.Fatal(err)
	}
	tr.Close()

	// 重新打开应自动补列，且老数据可查
	tr2, err := Open(dbPath, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Close()
	recs, err := tr2.QueryVisits(10, Filter{})
	if err != nil {
		t.Fatalf("迁移后查询失败: %v", err)
	}
	if len(recs) != 1 || recs[0].IP != "1.2.3.4" {
		t.Fatalf("老数据异常: %+v", recs)
	}
	tr2.Record(Visit{Time: time.Now(), IP: "10.0.0.1", Method: "GET", Path: "/", Code: "600519", Source: "ths", CacheHit: true, Status: 200})
	waitRows(t, tr2, 2)
}

func TestResolveGeoWithoutXDB(t *testing.T) {
	cache := make(map[string][3]string)
	for _, ip := range []string{"127.0.0.1", "10.1.0.12", "192.168.0.1", "169.254.1.1"} {
		if got := resolveGeo(cache, nil, ip); got[0] != "内网" {
			t.Errorf("resolveGeo(%s) = %v, want 内网", ip, got)
		}
	}
	// 无 xdb 时公网 IP 与非法 IP 均留空
	for _, ip := range []string{"223.5.5.5", "not-an-ip"} {
		if got := resolveGeo(cache, nil, ip); got != [3]string{} {
			t.Errorf("resolveGeo(%s) = %v, want 全空", ip, got)
		}
	}
}

func TestOpenEmptyPathDisabled(t *testing.T) {
	tr, err := Open("", "", 0)
	if err != nil || tr != nil {
		t.Fatalf("空 dbPath 应返回 (nil, nil): %v %v", tr, err)
	}
}
