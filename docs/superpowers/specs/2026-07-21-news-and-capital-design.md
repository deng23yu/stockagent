# 宏观快讯 + 个股资金面板 设计

日期：2026-07-21
状态：用户已确认（"分析需求→开发→验证→提交"）

## 需求分析与数据源结论（已实测）

| 需求 | 数据源 | 结论 |
|---|---|---|
| 宏观消息 | 新浪 7×24 快讯 `zhibo.sina.com.cn/api/zhibo/feed` | ✅ 可用，全文在响应里，点击即读（无需外链）。东财快讯需 req_trace 反爬参数，财联社返回 HTML，均弃用 |
| 融资融券 | 东财 `datacenter-web` RPTA_WEB_RZRQ_GGMX | ✅ 可用，日级：融资/融券余额、融资买入、占比、5日买入 |
| 净流入/流出 | 东财 `push2delay` fflow/daykline | ✅ 可用（实时 quote 接口的 f62 资金流字段在 delay 域名被置零不可用，fflow 日 K 正常）：主力/超大/大/中/小单每日净流入 |
| 北上资金 | — | ⚠️ **实时北向额度 2024 年 8 月起已被交易所停止披露，全网均无**。替代：`datacenter-web` RPT_MUTUAL_HOLDSTOCKNORTH_STA 沪深港通持股（股数/市值/占流通股比例，披露滞后，最新 2026-06-30），标注披露日期 |

## 设计

**`GET /api/v1/news`**（公开）：服务端拉新浪快讯 30 条，解析 `【标题】+正文`
（无【】的截前 30 字为题），缓存 5 分钟。返回 `{updated_at, items:[{id,time,title,content}]}`。
新包 `internal/news`（~70 行，URL 变量可 mock）。

**`GET /api/v1/capital?code=`**（公开）：并行拉三项，单项失败降级为 null：
`margin`（两融最新日）、`fund_flow`（近 5 日逐日五档净流入）、`northbound`（沪深港通持股+披露日）。
120s 内存缓存（按代码）。实现于 `internal/eastmoney/capital.go`，复用
`Client.get`（fflow 走 push2→push2delay 自动回退）。

**前端**
- 主页（idle 态）："宏观快讯"卡片区，列表 8 条（标题 + 相对时间），点击手风琴展开全文，
  手动刷新按钮。
- 报告页：ReportHeader 之后插入资金面板 —— 融资余额/融券余额/今日主力净流入（红绿）/
  5 日主力净流入 四张数字卡 + 近 5 日主力净流入迷你柱图（SVG 红绿柱）+
  沪深港通持股行（比例 + "MM-DD 披露"滞后标注）。金额统一格式化 亿/万。
- 面板独立 fetch（不塞进报告 JSON），保证资金数据不被 15 分钟报告缓存冻住。

## 测试

- news: 标题解析（带/不带【】）、错误处理（mock）。
- eastmoney: 三个资本接口解析（httptest mock，含 datacenter 分页结构与 fflow 记录序）。
- server: /api/v1/news 缓存只打一次上游；/api/v1/capital 全成功与部分失败降级。
- 前端构建 + 真实接口冒烟 + 线上验证，最后 git 提交推送。
