# stockagent

**A 股 AI 投研多智能体 CLI** —— 一条命令，4 位 AI 分析师并行工作，输出一份中文投资研究报告。

受 [ai-hedge-fund](https://github.com/virattt/ai-hedge-fund) 启发，但面向 **A 股**、**单二进制分发**、**数据免 key**:

| | ai-hedge-fund | stockagent |
| --- | --- | --- |
| 市场 | 美股 | **A 股** (沪深) |
| 运行时 | Python 环境 | **Go 单二进制** |
| 行情数据 | 需 API key | **东财/同花顺双数据源，均免 key** |
| 报告语言 | 英文 | **中文** |

> ⚠️ **免责声明**: 本项目仅供学习与技术研究，所有输出均由 AI 生成，**不构成任何投资建议**。股市有风险，投资需谨慎。

## 演示

```console
$ stockagent analyze 600519

贵州茅台 (600519) · AI 投研报告
────────────────────────────────────────────────────────
现价 1253.00  -0.48%    PE 14.37  PB 6.64  总市值 1.57万亿元
生成于 2026-07-18 12:00 · 模型 deepseek-chat · 耗时 12s

智能体          信号    置信度
技术面分析师    看多    72
基本面分析师    中性    55
消息面分析师    中性    50
风控官          看空    61

综合结论: 中性 (置信度 55)
────────────────────────────────────────────────────────
技术面均线多头排列但 RSI 偏高，估值处于历史中枢，消息面无
重大催化，波动率温和但当前位置风险收益比一般，建议观望……

关键要点:
  1. MA20/MA60 多头排列，中期趋势未破坏
  2. 动态 PE 14.4 处于白酒板块合理区间
  3. 距 52 周高点回撤约 20%，追高风险有限但动能不足

风险提示: 本报告由 AI 自动生成，仅供学习与技术研究……
```

*(以上为演示样例输出)*

## 工作原理

```
┌──────────────┐   K线/快照/公告 (东方财富/同花顺, 免 key, 并行拉取)
│  数据层       │ ──────────────────────────────────────
└──────────────┘
       │
┌──────────────┐   MA/MACD/RSI/波动率/最大回撤 (本地确定性计算)
│  指标层       │
└──────────────┘
       │
       ├─ 技术面分析师 ─┐
       ├─ 基本面分析师 ─┤   4 个 agent 并行 (errgroup)
       ├─ 消息面分析师 ─┤   各自输出 {signal, confidence, reasoning}
       └─ 风控官       ─┘
              │
       ┌──────────────┐
       │  组合经理     │  汇总分歧 → 最终结论 + 置信度 + 关键要点
       └──────────────┘
              │
     终端报告 / Markdown / JSON
```

设计要点:

- **指标本地算，LLM 只做解读** —— 数值计算确定性、可单测，LLM 幻觉不污染数据
- **降级容错** —— 公告拉取失败、单个分析师失败、组合经理 LLM 失败均不阻断流程，自动降级为本地加权聚合
- **A 股习惯** —— 红涨绿跌配色、万亿/亿元市值格式化、公告情绪分析

## 安装

```bash
# 方式一: 从源码安装 (Go ≥ 1.25)
git clone https://github.com/deng23yu/stockagent.git
cd stockagent && go install .

# 方式二: 下载 Release 预编译二进制 (linux / macOS / Windows)
```

> module 路径已配置为 `github.com/deng23yu/stockagent`，发布后可直接 `go install github.com/deng23yu/stockagent@latest`。

## 配置

只需一个 OpenAI 兼容协议的 LLM key。复制示例配置:

```bash
cp stockagent.yaml.example stockagent.yaml   # 或放 ~/.stockagent.yaml
```

```yaml
llm:
  base_url: https://api.deepseek.com   # 任意 OpenAI 兼容端点
  api_key: sk-your-api-key
  model: deepseek-chat
```

已验证兼容: DeepSeek / 通义千问 / Kimi / OpenAI / Ollama (本地零成本)。
也可用环境变量: `STOCKAGENT_BASE_URL` / `STOCKAGENT_API_KEY` / `STOCKAGENT_MODEL`。

## 用法

```bash
stockagent analyze 600519                          # 分析贵州茅台
stockagent analyze 600519 --source ths             # 改用同花顺数据源
stockagent analyze 300750                          # 分析宁德时代
stockagent analyze 000001 --format markdown -o report.md
stockagent analyze 600519 --format json            # 供程序消费
stockagent analyze 600519 --model deepseek-reasoner
```

| Flag | 默认 | 说明 |
| --- | --- | --- |
| `--days` | 250 | 拉取的日 K 线数量 |
| `--ann` | 20 | 拉取的公告数量 |
| `--source` | eastmoney | 行情数据源: `eastmoney` (东方财富) / `ths` (同花顺) |
| `--format` | terminal | `terminal` / `markdown` / `json` |
| `-o, --output` | stdout | 输出到文件 |
| `--model` / `--base-url` / `--api-key` | - | 覆盖配置 |

## Web UI (内嵌前端)

`stockagent serve` 启动后直接访问 `http://127.0.0.1:8080/` —— 单页应用已内嵌进二进制，无需额外部署。

*(截图待补充)*

特性: Kimi 官网风浅色设计、搜索即搜即得、分析师分步加载动画、信号卡/置信度条/综合结论卡片、缓存结果徽章。
技术栈: Vite + React + TypeScript + Tailwind CSS v4，产物经 `go:embed` 嵌入 (源码在 `web/`，改动后 `npm run build` 并提交 `web/dist`)。

## HTTP API (服务端模式)

```bash
stockagent serve --port 8080    # 默认监听 127.0.0.1:8080
```

- `GET /api/v1/analyze?code=600519&source=ths` — 分析接口，返回与 `--format json` 一致的 JSON
- `GET /api/v1/compare?codes=600519,000001` — 多股对比 (2~4 只并行分析，单只失败内联返回)
- `GET /api/v1/market` — 主要指数行情 (上证/深成/创业板，服务端缓存 60s)
- `GET /api/v1/hot-searches?days=7` — 热门搜索代码榜 (公开，来自访客库统计)
- `GET /api/v1/access-log?limit=50` — 最近 analyze 访问记录 (IP/股票代码/数据源/缓存命中/状态/耗时)，数据来自访客库
- `GET /api/v1/visits?limit=50&ip=&code=` — 访客记录 (时间/IP/归属地省市/路径/搜索内容/状态/耗时/UA)，需 admin token
- `GET /api/v1/visits/stats` — 访客聚合统计 (今日/累计 PV/UV、Top 归属地、Top 搜索代码)，需 admin token
- `GET /healthz` — 健康检查

特性: 结果缓存 15 分钟 (`--cache-ttl`，重复请求毫秒级返回，内存缓存最多 200 条)、并发上限 4 (超出返回 429)、CORS 全开 (前端开发友好)。
LLM key 只存在于服务端，前端永远接触不到。

### 访客记录 (SQLite + IP 归属地)

所有访客请求 (页面访问 + API 调用，自动跳过静态资源与 `/healthz`) 由中间件异步批量写入 SQLite，
不增加请求延迟。记录字段: 时间、IP、国家/省/市、方法、路径、原始 query (搜索内容)、
解析出的股票代码与数据源、缓存命中、状态码、耗时、UA。Web UI 右上角"访客记录"入口可直接查看
(含 PV/UV 统计与 IP/代码过滤)。

IP 省/市由 [ip2region](https://github.com/lionsoul2014/ip2region) 离线库解析，需下载数据文件
(默认读取 `./ip2region.xdb`，可用 `--ipdb` 指定；文件缺失时归属地留空，内网 IP 仍标记为"内网"):

```bash
curl -L -o ip2region.xdb https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v4.xdb
stockagent serve --host 0.0.0.0 --port 8080 --db visits.db --ipdb ip2region.xdb --admin-token <随机串>
```

访客相关 flag:

| flag | 默认值 | 说明 |
|---|---|---|
| `--db` | `visits.db` | SQLite 路径，置空禁用访客记录 |
| `--ipdb` | `ip2region.xdb` | ip2region 数据文件路径 |
| `--retention-days` | 30 | 访客记录保留天数 (0 = 永久) |
| `--admin-token` | 空 | `/api/v1/visits*` 的访问 token，置空不鉴权 (**公网暴露务必设置**) |
| `--trust-proxy` | false | 从 X-Forwarded-For/X-Real-IP 取访客 IP，仅在反向代理之后开启 |

```javascript
// 前端调用
fetch("/api/v1/analyze?code=600519&source=ths")
  .then(r => r.json())
  .then(report => console.log(report.final.signal, report.final.summary));
```

生产环境暴露: `--host 0.0.0.0` 并置于反向代理之后 (自行加鉴权/限流)。

## 开发

```bash
go test ./...            # 单元测试 + mock 端到端测试 (无需 key)
STOCKAGENT_LIVE=1 go test ./internal/eastmoney/ -run Live   # 真实接口回归
```

直接依赖仅 4 个: cobra / yaml.v3 / x/sync / tablewriter。

## Roadmap

- [ ] 北交所、港股/美股支持 (东财同接口延伸)
- [ ] F10 深度基本面 (财报核心指标)
- [ ] `watch` 子命令: 自选股批量分析
- [ ] 新闻舆情 (当前为公告)
- [ ] Homebrew tap / Scoop 分发

## License

[MIT](LICENSE)
