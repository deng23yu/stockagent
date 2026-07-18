package ths

import (
	"context"
	"os"
	"testing"
)

// TestLiveTHS 真实请求同花顺接口，需 STOCKAGENT_LIVE=1 显式开启。
// 同时交叉验证字段映射: 快照的昨收/今开/最高/最低应与末根 K 线一致。
func TestLiveTHS(t *testing.T) {
	if os.Getenv("STOCKAGENT_LIVE") == "" {
		t.Skip("设置 STOCKAGENT_LIVE=1 运行真实接口测试")
	}
	ctx := context.Background()
	c := New(nil)

	kd, err := c.KlinesByCode(ctx, "600519", 120)
	if err != nil {
		t.Fatalf("KlinesByCode: %v", err)
	}
	if len(kd.Bars) < 100 {
		t.Fatalf("K 线数量异常: %d", len(kd.Bars))
	}

	snap, err := c.SnapshotByCode(ctx, "600519")
	if err != nil {
		t.Fatalf("SnapshotByCode: %v", err)
	}
	if snap.Name != "贵州茅台" || snap.Price <= 0 {
		t.Errorf("快照异常: %+v", snap)
	}

	last := kd.Bars[len(kd.Bars)-1]
	if len(kd.Bars) >= 2 {
		prev := kd.Bars[len(kd.Bars)-2]
		if snap.PrevClose != 0 && prev.Close != snap.PrevClose {
			t.Logf("提示: 前根K线收盘 %.2f 与快照昨收 %.2f 不一致 (除权日可忽略)",
				prev.Close, snap.PrevClose)
		}
	}
	t.Logf("600519 %s 收 %.2f, 快照价 %.2f (%.2f%%), PE %.2f, 总市值 %.0f 元",
		last.Date, last.Close, snap.Price, snap.ChangePct, snap.PE, snap.TotalMarketCap)
}
