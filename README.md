# stockagent

**A 股 AI 投研多智能体 CLI** —— 一条命令，4 位 AI 分析师并行工作，输出一份中文投资研究报告。

受 [ai-hedge-fund](https://github.com/virattt/ai-hedge-fund) 启发，但面向 **A 股**、**单二进制分发**、**数据免 key**:

| | ai-hedge-fund | stockagent |
| --- | --- | --- |
| 市场 | 美股 | **A 股** (沪深) |
| 运行时 | Python 环境 | **Go 单二进制** |
| 行情数据 | 需 API key | **东方财富免 key** |
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
┌──────────────┐   K线/快照/公告 (东方财富, 免 key, 并行拉取)
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
git clone https://github.com/OWNER/stockagent.git
cd stockagent && go install .

# 方式二: 下载 Release 预编译二进制 (linux / macOS / Windows)
```

> 发布后请将 `OWNER` 替换为你的 GitHub 用户名，并把 `go.mod` 的 module 路径改为 `github.com/OWNER/stockagent`，即可支持 `go install github.com/OWNER/stockagent@latest`。

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
stockagent analyze 300750                          # 分析宁德时代
stockagent analyze 000001 --format markdown -o report.md
stockagent analyze 600519 --format json            # 供程序消费
stockagent analyze 600519 --model deepseek-reasoner
```

| Flag | 默认 | 说明 |
| --- | --- | --- |
| `--days` | 250 | 拉取的日 K 线数量 |
| `--ann` | 20 | 拉取的公告数量 |
| `--format` | terminal | `terminal` / `markdown` / `json` |
| `-o, --output` | stdout | 输出到文件 |
| `--model` / `--base-url` / `--api-key` | - | 覆盖配置 |

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
