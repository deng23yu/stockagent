# 市场概览条 UI 升级：图形化指数卡片

日期：2026-07-20
状态：用户已确认方向（弱化文字、多用图形、提升舒适感）

## 现状与问题

`/api/v1/market` 只返回点位与涨跌幅，前端是一行 12px 纯文字，信息密度低、无图形感。

## 设计

**后端**：`IndexQuote` 增加 `closes []float64`（近 30 个交易日收盘价）。
- 来源：腾讯行情 `web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=<symbol>,day,,,30,qfq`
  （新包 `internal/tencent`；东财 K 线在本机被封，delay 域名 K 线为空，腾讯云访问 gtimg 快）。
- secid→symbol 映射：`1.000001→sh000001`、`0.399001→sz399001`。
- `handleMarket` 改为每个指数并行拉取：报价（东财，带 delay 回退）+ 收盘价序列（腾讯，
  best-effort，失败则省略 closes）；沿用 60s 内存缓存。

**前端**：`MarketBar` 重写为三卡片网格（`grid-cols-3`）：
- 卡片：指数名（小字 ink-3）+ 涨跌 pill（红/绿 soft 底 + TrendingUp/Down 箭头图标 + 百分比）
- 大号点位（tnum、按涨跌着色）
- SVG sparkline：30 日走势折线 + 渐变面积填充 + 末端圆点，纯 SVG 手绘不引图表库
- 保持每 60s 自刷、失败静默隐藏

## 测试

- tencent: 解析 day/qfqday 两种键、bad params 报错（httptest mock）
- server: TestMarket 扩展 mock 腾讯 URL，断言 closes 字段
- 前端构建 + 冒烟
