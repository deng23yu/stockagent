# 亮点三件套：实时动态流 / 信号战绩埋点 / 多空辩论赛

日期：2026-07-20
状态：用户已确认按 3→2→1 顺序实施

## 功能 3：全站实时动态流（公开）

- 接口：`GET /api/v1/activity?limit=15`，公开。
- 数据源：visits 表 `code != ''` 的最近记录。compare 请求按 (ip, raw_query) 合并为
  一条多代码动态；analyze 一条一码。
- 脱敏：只暴露城市级归属地（province 字段，国外显示 country）与行为，绝不返回 IP/UA。
  返回 `[{time, city, action: "analyze"|"compare", codes: [...]}]`。
- 前端：搜索区下方一条滚动播报（脉冲红点 + 单条轮播，5s 切换，淡入淡出），
  文案如 "3 分钟前 · 南京股友 分析了 600737"。60s 拉新一次。

## 功能 2：AI 信号战绩埋点（静默积累，暂不展示）

- track 库新增两表（initSchema 自动迁移）：
  - `signals`：每次真实分析的信号事件 — id, time, code, name, signal, confidence, price。
  - `daily_closes`：tracked 代码每日收盘 — code, date, close，UNIQUE(code, date)。
- 埋点：`analyzeStock` 缓存未命中且 pipeline 成功时 `go RecordSignal(...)`（异步，不阻塞响应）。
- 每日快照：server 启动时检查 + 每交易日 15:35（A 股收盘后）为近 45 天有信号的代码
  抓当日收盘（tencent.DailyCloses，股票代码映射 6→sh / 0,3→sz）。
- 战绩榜展示（胜率/收益曲线）属后续迭代，数据攒够再做。

## 功能 1：多空辩论赛

- pipeline 拆分：`PrepareContext`（拉数据+算指标，供 analyze 与 debate 复用），
  `Run` 变为 PrepareContext + 分析师 + 汇总，行为不变。
- 新包 `internal/debate`：多方开场 → 空方开场 → 多方反驳 → 空方反驳 → 裁判裁决，
  5 次串行 LLM 调用（每轮可见前文）。返回
  `{code, name, turns: [{role: bull|bear, round, content}], verdict: {winner, bull_score, bear_score, reasoning}}`。
- 接口：`GET /api/v1/debate?code=`，共享 sem 并发上限与结果缓存（key `debate:<code>`）。
- 前端：SearchBar 增加"多空辩论"模式；DebateView 聊天式气泡（多方居左红、空方居右绿），
  逐条延迟揭示（1.2s/条），最后裁判卡（胜负 + 双方比分条 + 理由）。

## 测试

- track: signals/daily_closes 建表迁移、RecordSignal 去重追加、活动流查询与 compare 合并。
- debate: mock LLM 按角色提示词返回不同内容，验证 5 轮顺序与 verdict JSON 解析容错。
- server: /api/v1/activity 公开且不含 IP、/api/v1/debate 成功与缓存、信号落库触发。
- 前端构建 + 冒烟。

## 明确不做

- 辩论不做 SSE 流式（前端用延迟揭示模拟）；战绩榜展示页；辩论信号计入战绩。
