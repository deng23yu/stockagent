# 主页三功能：热门搜索榜 / 市场概览条 / 多股对比

日期：2026-07-20
状态：用户已确认实施（另含两项前置热修：DeepSeek key、eastmoney 封禁回退）

## 背景

- LLM 已从 Kimi 切换为 DeepSeek（`/etc/stockagent/stockagent.yaml` 与 `~/.stockagent.yaml`，已验证）。
- 东财 `push2`/`push2his` 主域名对本机 IP 连接重置（疑似机房 IP 限流）。
  已上线两处回退：① 客户端级 主域名→`push2delay`（快照可用，K 线 delay 域名为空）；
  ② pipeline 级 K 线失败→ths 数据源兜底（公告仍走东财公告子域，未被封）。

## 功能一：热门搜索榜（公开）

- 后端：`track.QueryHotCodes(days, limit)` ——
  `SELECT code, COUNT(*) FROM visits WHERE code!='' AND time>=cutoff GROUP BY code ORDER BY count DESC`。
- 接口：`GET /api/v1/hot-searches?days=7&limit=10` → `{"items":[{name,code? no: name=code,count}]}`
  返回 `[{name:"600737",count:3}]`。**不鉴权**（首页公开组件）；tracker 未启用时返回空数组（不报错，
  首页组件静默隐藏）。
- 前端：搜索框下方一行 "🔥 热门搜索" chips（代码 × 次数），点击 chip 直接触发该代码分析。

## 功能二：市场概览条（公开）

- 后端：eastmoney 新增 `IndexQuotes(ctx)`，逐个调用现有 quote 接口（f43 价格、f58 名称、
  f170 涨跌幅，均 ×100 缩放），指数 secid：`1.000001` 上证、`0.399001` 深成、`0.399006` 创业板。
  复用客户端的 delay 域名回退。
- 接口：`GET /api/v1/market` → `{"updated_at":..., "indices":[{name,code,price,change_pct}]}`，
  服务端内存缓存 60s（避免每个访客都打上游）。
- 前端：header 下方市场条，指数名 + 点位 + 涨跌幅（红涨绿跌，tnum 等宽数字），
  每 60s 自刷，失败静默隐藏。

## 功能三：多股对比（限 4 只）

- 接口：`GET /api/v1/compare?codes=600519,600737,000001&source=`。
  - 校验：逗号分隔、去重、每只 6 位数字，数量 2~4，否则 400。
  - 执行：每代码一个 goroutine，先查与 analyze 共享的结果缓存，未命中走 `pipeline.Run`
    （并发信号量 maxConcurrent=4 天然排队）；单代码失败不影响其他，错误内联返回。
  - 响应：`{"items":[{"code","ok":true,"report":{...}} | {"code","ok":false,"error":"..."}]}`。
- 前端：搜索框上方加 [单股 | 对比] 切换；对比模式输入逗号分隔的 2~4 个代码；
  结果以卡片网格（sm:grid-cols-2）展示：名称/代码、现价/涨跌幅、四位分析师信号点、
  综合信号 + 置信度 + 摘要（3 行截断）；失败代码显示错误卡片。
- 成本说明：N 只 = N 次完整分析（各 5 次 LLM 调用），靠共享缓存与 4 只上限控制。

## 测试

- track: QueryHotCodes（天数窗口、排序、空 code 排除）。
- eastmoney: IndexQuotes 解析（httptest mock）。
- server: hot-searches（公开访问、tracker nil 返回空）、market（缓存生效只打一次上游）、
  compare（参数校验 400、两只成功一只失败混合、缓存复用）。
- 前端：`npm run build`（tsc）+ 页面冒烟。

## 明确不做

- 不做报告分享页 / 胜率追踪 / 定时扫描（后续迭代）；指数暂不加沪深 300；
  对比不做"最优标的高亮"等花哨逻辑。
