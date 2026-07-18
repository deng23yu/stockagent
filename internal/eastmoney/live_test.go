package eastmoney

import (
	"context"
	"os"
	"testing"
)

// TestLiveEastmoney 真实请求东方财富接口，需 STOCKAGENT_LIVE=1 显式开启。
// CI 默认跳过，用于本地验证接口字段未变更。
func TestLiveEastmoney(t *testing.T) {
	if os.Getenv("STOCKAGENT_LIVE") == "" {
		t.Skip("设置 STOCKAGENT_LIVE=1 运行真实接口测试")
	}
	ctx := context.Background()
	c := New(nil)

	kd, err := c.Klines(ctx, "1.600519", 120)
	if err != nil {
		t.Fatalf("Klines: %v", err)
	}
	if kd.Name != "贵州茅台" || len(kd.Bars) < 100 {
		t.Errorf("K 线数据异常: name=%q bars=%d", kd.Name, len(kd.Bars))
	}

	snap, err := c.Snapshot(ctx, "1.600519")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Name != "贵州茅台" || snap.Price <= 0 {
		t.Errorf("快照异常: %+v", snap)
	}

	anns, err := c.Announcements(ctx, "600519", 5)
	if err != nil {
		t.Fatalf("Announcements: %v", err)
	}
	if len(anns) == 0 {
		t.Error("公告为空")
	}
}
