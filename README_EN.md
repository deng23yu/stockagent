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

## Web UI (embedded frontend)

Once `stockagent serve` is running, open `http://127.0.0.1:8080/` — the single-page app is embedded in the binary, nothing else to deploy.

*(screenshot to be added)*

Highlights: Kimi-style light design, staged analyst progress animation, signal cards with confidence bars, verdict card, cached-result badge.
Stack: Vite + React + TypeScript + Tailwind CSS v4, embedded via `go:embed` (source in `web/`; after changes run `npm run build` and commit `web/dist`).

## HTTP API (server mode)

```bash
stockagent serve --port 8080    # listens on 127.0.0.1:8080 by default
```

- `GET /api/v1/analyze?code=600519&source=ths` — analysis endpoint, same JSON as `--format json`
- `GET /api/v1/compare?codes=600519,000001` — multi-stock compare (2-4 codes analyzed in parallel, per-code errors inlined)
- `GET /api/v1/market` — major index quotes (SSE/SZSE/ChiNext, 60s server-side cache)
- `GET /api/v1/hot-searches?days=7` — hot searched codes (public, aggregated from the visitor DB)
- `GET /api/v1/access-log?limit=50` — recent analyze access records (IP/code/source/cache-hit/status/latency), served from the visitor DB
- `GET /api/v1/visits?limit=50&ip=&code=` — visitor records (time/IP/geo province+city/path/search query/status/latency/UA), requires admin token
- `GET /api/v1/visits/stats` — visitor aggregates (today/total PV/UV, top regions, top searched codes), requires admin token
- `GET /healthz` — health check

Features: 15-minute result cache (`--cache-ttl`, repeat requests return in milliseconds, capped at 200 entries),
concurrency cap of 4 (429 beyond that), permissive CORS for local frontend development.
The LLM key stays server-side and is never exposed to clients.

### Visitor tracking (SQLite + IP geolocation)

Every visitor request (page loads + API calls; static assets and `/healthz` are skipped) is
batch-written to SQLite asynchronously by middleware — zero added request latency. Fields: time, IP,
country/province/city, method, path, raw query (search content), parsed stock code and source,
cache hit, status, latency, user agent. The Web UI has a "访客记录" entry in the header
(with PV/UV stats and IP/code filters).

Geo lookup uses the offline [ip2region](https://github.com/lionsoul2014/ip2region) database.
Download the data file (default path `./ip2region.xdb`, override with `--ipdb`; if missing,
geo fields stay empty and internal IPs are still marked as "内网"):

```bash
curl -L -o ip2region.xdb https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v4.xdb
stockagent serve --host 0.0.0.0 --port 8080 --db visits.db --ipdb ip2region.xdb --admin-token <random>
```

Visitor-related flags:

| flag | default | description |
|---|---|---|
| `--db` | `visits.db` | SQLite path, empty to disable tracking |
| `--ipdb` | `ip2region.xdb` | ip2region data file path |
| `--retention-days` | 30 | visitor record retention in days (0 = forever) |
| `--admin-token` | empty | token for `/api/v1/visits*`; empty = no auth (**set this when publicly exposed**) |
| `--trust-proxy` | false | take visitor IP from X-Forwarded-For/X-Real-IP, enable only behind a reverse proxy |

```javascript
fetch("/api/v1/analyze?code=600519&source=ths")
  .then(r => r.json())
  .then(report => console.log(report.final.signal, report.final.summary));
```

For production exposure use `--host 0.0.0.0` behind a reverse proxy (add your own auth/rate-limiting).

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
