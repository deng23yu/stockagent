package eastmoney

import "context"

// Source 抽象行情数据源 (K线 + 快照)。
// 实现: 东方财富 (本包 Client)、同花顺 (internal/ths Client)。
type Source interface {
	KlinesByCode(ctx context.Context, code string, limit int) (*KlineData, error)
	SnapshotByCode(ctx context.Context, code string) (*Snapshot, error)
}

// KlinesByCode 按 6 位股票代码拉取 K 线 (内部完成 SecID 映射)。
func (c *Client) KlinesByCode(ctx context.Context, code string, limit int) (*KlineData, error) {
	secid, err := SecID(code)
	if err != nil {
		return nil, err
	}
	return c.Klines(ctx, secid, limit)
}

// SnapshotByCode 按 6 位股票代码拉取快照 (内部完成 SecID 映射)。
func (c *Client) SnapshotByCode(ctx context.Context, code string) (*Snapshot, error) {
	secid, err := SecID(code)
	if err != nil {
		return nil, err
	}
	return c.Snapshot(ctx, secid)
}
