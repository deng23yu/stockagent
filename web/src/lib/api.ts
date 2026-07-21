// 与后端 report/json.go 的 schema 对齐。
export interface Snapshot {
  code: string
  name: string
  price: number
  open: number
  high: number
  low: number
  prev_close: number
  change_pct: number
  turnover_pct: number
  pe: number
  pb: number
  total_market_cap: number
  float_market_cap: number
}

export type Signal = 'bullish' | 'bearish' | 'neutral'

export interface AgentResult {
  agent: string
  signal: Signal | string
  confidence: number
  reasoning: string
  err?: string
}

export interface FinalVerdict {
  signal: Signal | string
  confidence: number
  summary: string
  key_points: string[]
  degraded: boolean
}

export interface Report {
  code: string
  name: string
  generated_at: string
  model: string
  snapshot: Snapshot
  results: AgentResult[]
  final: FinalVerdict
  disclaimer: string
}

export async function analyzeStock(
  code: string,
  source: string,
  signal?: AbortSignal,
): Promise<{ report: Report; cached: boolean }> {
  const resp = await fetch(
    `/api/v1/analyze?code=${encodeURIComponent(code)}&source=${encodeURIComponent(source)}`,
    { signal },
  )
  const body = await resp.json().catch(() => null)
  if (!resp.ok) {
    throw new Error(body?.error ?? `请求失败 (HTTP ${resp.status})`)
  }
  return { report: body as Report, cached: resp.headers.get('X-Cache') === 'hit' }
}

// 与后端 track.Visit 的 JSON 字段对齐。
export interface Visit {
  id: number
  time: string
  ip: string
  method: string
  path: string
  query?: string
  code?: string
  source?: string
  cache_hit: boolean
  status: number
  latency_ms: number
  user_agent?: string
  country?: string
  province?: string
  city?: string
}

// 与后端 track.Stats 的 JSON 字段对齐。
export interface VisitStats {
  today_pv: number
  today_uv: number
  total_pv: number
  total_uv: number
  top_cities: { name: string; count: number }[]
  top_codes: { name: string; count: number }[]
}

// AuthError 表示管理接口 401 (需要 admin token)。
export class AuthError extends Error {
  constructor() {
    super('需要 admin token')
    this.name = 'AuthError'
  }
}

async function getJSON(url: string, token?: string, signal?: AbortSignal) {
  const resp = await fetch(url, {
    signal,
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (resp.status === 401) throw new AuthError()
  const body = await resp.json().catch(() => null)
  if (!resp.ok) {
    throw new Error(body?.error ?? `请求失败 (HTTP ${resp.status})`)
  }
  return body
}

export async function fetchVisits(
  opts: { limit?: number; ip?: string; code?: string; token?: string; signal?: AbortSignal } = {},
): Promise<Visit[]> {
  const params = new URLSearchParams()
  params.set('limit', String(opts.limit ?? 200))
  if (opts.ip) params.set('ip', opts.ip)
  if (opts.code) params.set('code', opts.code)
  const body = await getJSON(`/api/v1/visits?${params}`, opts.token, opts.signal)
  return (body?.records ?? []) as Visit[]
}

export async function fetchVisitStats(token?: string, signal?: AbortSignal): Promise<VisitStats> {
  return (await getJSON('/api/v1/visits/stats', token, signal)) as VisitStats
}

// 与后端 eastmoney.IndexQuote 对齐。
export interface IndexQuote {
  code: string
  name: string
  price: number
  change_pct: number
  closes?: number[]
}

export interface MarketData {
  updated_at: string
  indices: IndexQuote[]
}

export async function fetchMarket(signal?: AbortSignal): Promise<MarketData> {
  const resp = await fetch('/api/v1/market', { signal })
  if (!resp.ok) throw new Error(`行情失败 (HTTP ${resp.status})`)
  return (await resp.json()) as MarketData
}

export interface HotSearch {
  name: string
  count: number
}

export async function fetchHotSearches(days = 7, signal?: AbortSignal): Promise<HotSearch[]> {
  const resp = await fetch(`/api/v1/hot-searches?days=${days}`, { signal })
  const body = await resp.json().catch(() => null)
  if (!resp.ok) throw new Error(body?.error ?? `请求失败 (HTTP ${resp.status})`)
  return (body?.items ?? []) as HotSearch[]
}

// 多股对比单项: 成功含 report，失败含 error。
export interface CompareItem {
  code: string
  ok: boolean
  report?: Report
  error?: string
}

export async function compareStocks(
  codes: string[],
  source: string,
  signal?: AbortSignal,
): Promise<CompareItem[]> {
  const resp = await fetch(
    `/api/v1/compare?codes=${encodeURIComponent(codes.join(','))}&source=${encodeURIComponent(source)}`,
    { signal },
  )
  const body = await resp.json().catch(() => null)
  if (!resp.ok) {
    throw new Error(body?.error ?? `请求失败 (HTTP ${resp.status})`)
  }
  return (body?.items ?? []) as CompareItem[]
}

// 与后端 track.Activity 对齐 (已脱敏)。
export interface Activity {
  time: string
  city: string
  action: 'analyze' | 'compare'
  codes: string[]
}

export async function fetchActivity(limit = 15, signal?: AbortSignal): Promise<Activity[]> {
  const resp = await fetch(`/api/v1/activity?limit=${limit}`, { signal })
  const body = await resp.json().catch(() => null)
  if (!resp.ok) throw new Error(body?.error ?? `请求失败 (HTTP ${resp.status})`)
  return (body?.items ?? []) as Activity[]
}

// 与后端 debate.Debate 对齐。
export interface DebateTurn {
  role: 'bull' | 'bear'
  round: number
  content: string
}

export interface Debate {
  code: string
  name: string
  turns: DebateTurn[]
  verdict: {
    winner: 'bull' | 'bear' | 'draw'
    bull_score: number
    bear_score: number
    reasoning: string
  }
  model: string
  generated_at: string
}

export async function fetchDebate(code: string, source: string, signal?: AbortSignal): Promise<Debate> {
  const resp = await fetch(
    `/api/v1/debate?code=${encodeURIComponent(code)}&source=${encodeURIComponent(source)}`,
    { signal },
  )
  const body = await resp.json().catch(() => null)
  if (!resp.ok) {
    throw new Error(body?.error ?? `请求失败 (HTTP ${resp.status})`)
  }
  return body as Debate
}

// 与后端 news.Item 对齐。
export interface NewsItem {
  id: number
  time: string
  title: string
  content: string
}

export async function fetchNews(signal?: AbortSignal): Promise<{ updated_at: string; items: NewsItem[] }> {
  const resp = await fetch('/api/v1/news', { signal })
  const body = await resp.json().catch(() => null)
  if (!resp.ok) throw new Error(body?.error ?? `请求失败 (HTTP ${resp.status})`)
  return body
}

// 与后端 eastmoney 资金面板结构对齐。
export interface MarginData {
  date: string
  rzye: number
  rqye: number
  rzrqye: number
  rzmre: number
  rzyezb: number
  rzmre5d: number
}

export interface FundFlowDay {
  date: string
  main: number
  super_large: number
  large: number
  medium: number
  small: number
}

export interface NorthboundHold {
  date: string
  shares: number
  market_cap: number
  shares_ratio: number
}

export interface CapitalData {
  code: string
  margin: MarginData | null
  fund_flow: FundFlowDay[] | null
  northbound: NorthboundHold | null
}

export async function fetchCapital(code: string, signal?: AbortSignal): Promise<CapitalData> {
  const resp = await fetch(`/api/v1/capital?code=${encodeURIComponent(code)}`, { signal })
  const body = await resp.json().catch(() => null)
  if (!resp.ok) throw new Error(body?.error ?? `请求失败 (HTTP ${resp.status})`)
  return body as CapitalData
}
