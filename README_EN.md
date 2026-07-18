# stockagent

**AI multi-agent investment research CLI for China A-shares** — one command, four AI analysts working in parallel, producing a full research report.

Inspired by [ai-hedge-fund](https://github.com/virattt/ai-hedge-fund), but built for the **A-share market** (Shanghai & Shenzhen), distributed as a **single Go binary**, with **key-free market data** from Eastmoney & THS (dual sources).

[中文文档](README.md)

> ⚠️ **Disclaimer**: For educational and research purposes only. All output is AI-generated and does **not** constitute investment advice.

## How it works

```
Market data (Eastmoney / THS, no API key, fetched in parallel)
        │
Technical indicators (MA / MACD / RSI / volatility / max drawdown,
computed locally & deterministically — the LLM only interprets)
        │
  ┌─────┴──────┬──────────────┬──────────┐
Technical  Fundamental  Sentiment   Risk      ← 4 agents in parallel
 Analyst    Analyst     Analyst    Officer
  └─────┬──────┴──────────────┴──────────┘
        │
Portfolio Manager  ← synthesizes disagreements into a final verdict
        │
Terminal report / Markdown / JSON
```

Key design choices:

- **Numbers are computed locally, the LLM only interprets them** — deterministic, unit-testable, no hallucinated data.
- **Graceful degradation** — if announcements fetch fails, a single analyst fails, or the portfolio-manager LLM call fails, the pipeline degrades to a local confidence-weighted aggregation instead of crashing.
- **A-share conventions** — red-up/green-down colors, 亿/万亿 market-cap formatting, announcement-driven sentiment.

## Install

```bash
# From source (Go ≥ 1.25)
git clone https://github.com/deng23yu/stockagent.git
cd stockagent && go install .

# Or download a prebuilt binary from Releases (linux / macOS / Windows)
```

## Configure

One OpenAI-compatible LLM key is all you need:

```bash
cp stockagent.yaml.example stockagent.yaml   # or ~/.stockagent.yaml
```

```yaml
llm:
  base_url: https://api.deepseek.com   # any OpenAI-compatible endpoint
  api_key: sk-your-api-key
  model: deepseek-chat
```

Works with DeepSeek / Qwen / Kimi / OpenAI / Ollama (fully local, zero cost).
Env vars also supported: `STOCKAGENT_BASE_URL` / `STOCKAGENT_API_KEY` / `STOCKAGENT_MODEL`.

## Usage

```bash
stockagent analyze 600519                          # Kweichow Moutai
stockagent analyze 600519 --source ths             # use THS data source
stockagent analyze 300750 --format markdown -o report.md
stockagent analyze 000001 --format json
stockagent analyze 600519 --model deepseek-reasoner
```

| Flag | Default | Description |
| --- | --- | --- |
| `--days` | 250 | Number of daily K-line bars |
| `--ann` | 20 | Number of announcements |
| `--source` | eastmoney | Market data source: `eastmoney` / `ths` (THS) |
| `--format` | terminal | `terminal` / `markdown` / `json` |
| `-o, --output` | stdout | Write to a file |
| `--model` / `--base-url` / `--api-key` | - | Override config |

## Development

```bash
go test ./...            # unit tests + mocked end-to-end (no key needed)
STOCKAGENT_LIVE=1 go test ./internal/eastmoney/ -run Live   # live API regression
```

Only 4 direct dependencies: cobra / yaml.v3 / x/sync / tablewriter.

## Roadmap

- [ ] Beijing Stock Exchange, HK/US markets (same Eastmoney endpoints)
- [ ] Deep fundamentals via F10 financials
- [ ] `watch` command for batch analysis of a watchlist
- [ ] News sentiment (currently announcements)
- [ ] Homebrew tap / Scoop distribution

## License

[MIT](LICENSE)
