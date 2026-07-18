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
