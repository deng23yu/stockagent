# 前端"访客记录"入口设计

日期：2026-07-20
状态：已确认（auto 模式下由实现者决策）

## 目标

Web UI 增加入口查看 `/api/v1/visits` 的访客记录（时间/IP/归属地/搜索内容等）。

## 方案比选

- **头部按钮 + 模态面板——选用**。项目无路由（单视图 SPA），模态最简、
  不引入 react-router 依赖；风格沿用现有卡片/遮罩即可。
- 新增路由页：需要引入 react-router，对单页小应用过度设计，排除。
- 主页内嵌区块：访客记录是管理向功能，不应抢占主报告版面，排除。

## 设计

- 入口：header 右侧 GitHub 链接旁，"访客记录"文字按钮（lucide `ScrollText` 图标），
  样式同 GitHub 链接（`text-xs text-ink-3 hover:text-ink-2`）。
- 面板：模态（半透明遮罩 + 居中卡片，最大宽 4xl、高 80vh、内部滚动），
  打开即拉取 `GET /api/v1/visits?limit=200`，之后每 10s 自动刷新 + 手动"刷新"按钮；
  点遮罩或 ✕ 关闭，关闭时停止轮询。
- 表格列：时间（MM-DD HH:mm:ss）、IP、归属地（country/province/city 拼接去重）、
  请求（`METHOD path?query`，超长截断）、代码、状态（2xx 绿 / 4xx 黄 / 5xx 红）、
  耗时（ms）、UA（截断 + title 悬浮）。小屏横向滚动。
- 状态处理：加载中骨架 / 错误（含后端 503 未启用 `--db`）/ 空数据"暂无记录"。
- 改动文件：`web/src/lib/api.ts`（Visit 类型 + fetchVisits）、
  `web/src/components/VisitsPanel.tsx`（新）、`web/src/App.tsx`（按钮 + state）。
- 构建：`npm run build`（tsc + vite）后提交 `web/dist`，重编 Go 二进制使 embed 生效。
