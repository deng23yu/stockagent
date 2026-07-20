# 访客记录系统优化（安全/维护/功能/整洁 八项）

日期：2026-07-20
状态：用户已确认全部实施

## 范围与方案

### 安全

1. **管理接口鉴权**：`/api/v1/visits`、`/api/v1/visits/stats` 增加 token 校验
   （`Authorization: Bearer <token>` 或 `?token=`，ConstantTimeCompare）。
   serve 新增 `--admin-token`；为空则不鉴权（本地开发兼容），
   但 `--host 0.0.0.0` 且 token 为空时启动日志给出警告。前端面板 401 时显示
   token 输入框，存 localStorage 后自动重试。
2. **可信代理开关**：`clientIP` 仅在 `--trust-proxy` 开启时读取
   X-Forwarded-For / X-Real-IP，否则一律用 RemoteAddr，杜绝访客伪造来源 IP。

### 数据维护

3. **visits 表保留策略**：serve 新增 `--retention-days`（默认 30，0 表示不清理）。
   启动时及每 24h 由 worker goroutine 执行 `DELETE FROM visits WHERE time < cutoff`。
4. **分析缓存容量上限**：内存缓存最多 200 条，满时淘汰创建时间最早的一条
   （TTL 机制不变）。

### 功能

5. **访客面板过滤**：面板增加 IP / 股票代码过滤输入框，对接已有的
   `/api/v1/visits?ip=&code=` 参数。
6. **聚合统计**：新增 `GET /api/v1/visits/stats`（需鉴权），返回
   今日/累计 PV、UV（DISTINCT ip）、Top 归属地（country+province+city 组合，
   前端按现有 geoText 规则拼接）、Top 搜索代码（各取前 10）。
   面板顶部展示统计卡片。today 按本地零点（RFC3339 字符串比较，中国无 DST）。
7. **UA 短格式展示**：存储保留原始 UA，前端展示层解析为
   "Chrome · Windows" / "微信 · iOS" / "curl" / "爬虫" 等短格式。

### 工程整洁

8. **双日志合并 + 批量写入**：
   - 删除 `accesslog.go` 与 `--access-log` flag；`/api/v1/access-log`
     改从 SQLite 读（`path='/api/v1/analyze'`），返回 JSON 形状保持
     向后兼容（AccessRecord 字段不变）。
   - visits 表新增 `source`、`cache_hit` 两列（中间件从 query 参数与
     响应头 X-Cache 提取）；已存在的 DB 通过 `PRAGMA table_info` 检测 +
     `ALTER TABLE ADD COLUMN` 自动迁移。
   - track worker 改批量写入：攒满 50 条或 100ms 触发，单事务提交；
     Close 时先 flush 再关闭。

## 牵连改动

- `server.New` 参数收敛为 `Options{CacheTTL, Tracker, AdminToken, TrustProxy}`
  （原 accessLogPath 参数随合并删除）。
- `track.Open(dbPath, ipdbPath string, retentionDays int)`；
  `QueryVisits` 改收 `Filter{IP, Code, Path}`。
- 测试：`TestAccessLog` 重写为 SQLite 版（断言 JSON 兼容与 cache_hit）；
  新增 admin-token、trust-proxy、retention、stats 测试。
- README / README_EN 同步新 flags 与接口；部署 unit 追加
  `--admin-token <随机>` `--retention-days 30`（生产直连反代，不开 trust-proxy）。

## 明确不做

- 不改 analyze 主流程与报告 schema；不引入外部组件（Redis 等）；
  安全组放行属云控制台运维事项，不在代码侧处理。
