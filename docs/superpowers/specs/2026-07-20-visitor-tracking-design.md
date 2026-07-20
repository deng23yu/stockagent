# 访客访问记录（落库）设计

日期：2026-07-20
状态：已确认（auto 模式下由实现者决策）

## 背景与目标

stockagent 目前仅对 `/api/v1/analyze` 记录访问日志（JSONL 文件 + 内存环形缓冲，
见 `internal/server/accesslog.go`）。用户要求：**记录所有访客的访问记录和搜索内容，
包含 IP 地址、地市、时间等，存入数据库**，以便后续查询分析。

## 方案比选

### 数据库

- **SQLite（modernc.org/sqlite，纯 Go）——选用**。单文件、零运维；
  goreleaser 需交叉编译 linux/darwin/windows（`.goreleaser.yaml`），CGO 必须关闭，
  故排除 `mattn/go-sqlite3`。
- MySQL/PostgreSQL：需额外部署服务，对单机小工具过度设计，排除。
- 继续用 JSONL 文件：不满足"记录到数据库"的要求，排除。

### IP 地市解析

- **ip2region 本地 xdb 库——选用**。离线、微秒级查询、国内 IP 库权威。
  xdb 数据文件（约 11MB）不提交仓库，通过 `--ipdb` flag 指定路径；
  文件缺失时降级：地市字段留空（内网 IP 仍会标记为"内网"）。
- 在线 GeoIP API：依赖第三方可用性与频限，且会把访客 IP 发给第三方，排除。

## 架构

新增 `internal/track` 包，与现有 `accesslog.go`（analyze 专用审计）并存、互不影响：

```
HTTP 请求 → server.trackMiddleware（捕获 method/path/query/status/耗时/IP/UA）
          → Tracker.Record(Visit)（非阻塞，送入 buffered channel）
          → 后台单 writer goroutine：IP 归属地解析（带缓存）→ INSERT SQLite
```

- 记录范围：全部请求，**排除** `/healthz`、`/assets/` 下的前端静态资源、
  以及带文件扩展名的路径（`.js/.css/.png/...`），避免静态资源刷屏。
- 响应路径零阻塞：channel 满时丢弃该条并 `log.Printf` 告警，绝不阻塞访客请求。
- SQLite 连接 `SetMaxOpenConns(1)`，单 writer 天然规避 `SQLITE_BUSY`。

## 数据模型

表 `visits`：

| 列 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK AUTOINCREMENT | |
| time | TEXT (RFC3339) | 请求到达时间（本地时区） |
| ip | TEXT | 客户端 IP（沿用 clientIP：XFF → X-Real-IP → RemoteAddr） |
| method | TEXT | GET/POST... |
| path | TEXT | 请求路径，如 `/`、`/api/v1/analyze` |
| query | TEXT | 原始 query string（搜索内容，如 `code=600519&source=ths`） |
| code | TEXT | 解析出的 `code` 参数（股票代码，便于检索），无则空 |
| status | INTEGER | 响应状态码 |
| latency_ms | INTEGER | 处理耗时 |
| user_agent | TEXT | |
| country / province / city | TEXT | 归属地；内网 IP 记 country="内网"；无 xdb 时留空 |

索引：`idx_visits_time(time)`、`idx_visits_ip(ip)`。

## 接口与配置

- 新增 `GET /api/v1/visits?limit=50&ip=&code=`：从 DB 查询访客记录，新的在前，
  `limit` 上限 500，`ip`/`code` 为可选精确过滤。
- 现有 `GET /api/v1/access-log`（内存中 analyze 记录）保持不变，向后兼容。
- `serve` 新增 flag：
  - `--db`（默认 `visits.db`）：SQLite 路径，置空则禁用访客记录；
  - `--ipdb`（默认 `ip2region.xdb`）：ip2region 数据文件路径，不存在则地市留空。

## 错误处理

- DB 打开/建表失败：`serve` 启动直接报错退出（包装为 `访客数据库: ...`）。
- 运行期单条插入失败：`log.Printf` 告警，不影响后续请求。
- 优雅关闭：`Server.Close` 先停止接收新记录，带超时地 drain channel 后关闭 DB。

## 测试

- `internal/track`：临时目录建库 → Record 多条（含内网 IP）→ 轮询等待异步落库 →
  校验字段、排序（新的在前）、`ip`/`code` 过滤、Close 后数据完整。
- `internal/server`：挂接真实 tracker（临时 DB），请求 `/`（记录）、
  `/assets/x.js`（跳过）、`/healthz`（跳过）、`/api/v1/analyze` 无 code（记录 400），
  再查 `/api/v1/visits` 验证记录内容。
- 不依赖真实 xdb 文件（公网 IP 解析留空即可；内网判定走 `net/netip`，无需 xdb）。
