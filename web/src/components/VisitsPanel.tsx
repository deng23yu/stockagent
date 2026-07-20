import { useCallback, useEffect, useState } from 'react'
import { AlertTriangle, Lock, RefreshCw, Search, X } from 'lucide-react'
import { AuthError, fetchVisits, fetchVisitStats, type Visit, type VisitStats } from '../lib/api'

const TOKEN_KEY = 'stockagent.admin_token'

// VisitsPanel 以模态面板展示访客记录与聚合统计 (/api/v1/visits*)，
// 打开期间每 10s 自动刷新；服务端开启 --admin-token 时先要求输入 token。
export default function VisitsPanel({ onClose }: { onClose: () => void }) {
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) ?? '')
  const [needsAuth, setNeedsAuth] = useState(false)
  const [visits, setVisits] = useState<Visit[] | null>(null)
  const [stats, setStats] = useState<VisitStats | null>(null)
  const [error, setError] = useState('')
  const [refreshing, setRefreshing] = useState(false)
  // 过滤框草稿与已生效条件 (点查询/回车才生效)
  const [draftIp, setDraftIp] = useState('')
  const [draftCode, setDraftCode] = useState('')
  const [filter, setFilter] = useState({ ip: '', code: '' })

  const load = useCallback(async () => {
    setRefreshing(true)
    try {
      const [v, s] = await Promise.all([
        fetchVisits({ limit: 200, ip: filter.ip, code: filter.code, token }),
        fetchVisitStats(token),
      ])
      setVisits(v)
      setStats(s)
      setError('')
      setNeedsAuth(false)
    } catch (e) {
      if (e instanceof AuthError) setNeedsAuth(true)
      else setError((e as Error).message)
    } finally {
      setRefreshing(false)
    }
  }, [token, filter])

  useEffect(() => {
    load()
    const timer = setInterval(load, 10_000)
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      clearInterval(timer)
      window.removeEventListener('keydown', onKey)
    }
  }, [load, onClose])

  const submitToken = (t: string) => {
    localStorage.setItem(TOKEN_KEY, t)
    setToken(t)
  }

  const applyFilter = () => setFilter({ ip: draftIp.trim(), code: draftCode.trim() })
  const clearFilter = () => {
    setDraftIp('')
    setDraftCode('')
    setFilter({ ip: '', code: '' })
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4"
      onClick={onClose}
    >
      <div
        className="shadow-pop flex max-h-[85vh] w-full max-w-4xl flex-col rounded-2xl border border-line bg-white"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2.5 border-b border-line px-5 py-3.5">
          <div className="text-sm font-semibold tracking-tight">访客记录</div>
          <div className="text-xs text-ink-3">
            {visits ? `最近 ${visits.length} 条` : '加载中'} · 每 10s 自动刷新
          </div>
          <button
            className="ml-auto rounded-lg p-1.5 text-ink-3 transition-colors hover:bg-accent-soft hover:text-accent disabled:opacity-40"
            onClick={load}
            disabled={refreshing}
            title="刷新"
          >
            <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
          </button>
          <button
            className="rounded-lg p-1.5 text-ink-3 transition-colors hover:bg-accent-soft hover:text-accent"
            onClick={onClose}
            title="关闭 (Esc)"
          >
            <X size={14} />
          </button>
        </div>

        <div className="overflow-auto px-5 py-3">
          {needsAuth ? (
            <TokenGate onSubmit={submitToken} />
          ) : (
            <>
              {error && (
                <div className="my-2 flex items-start gap-2.5 rounded-2xl border border-bull/20 bg-bull-soft px-5 py-4 text-sm text-bull">
                  <AlertTriangle size={16} className="mt-0.5 shrink-0" />
                  <span>{error}</span>
                </div>
              )}
              {stats && <StatsSection stats={stats} />}
              <div className="flex items-center gap-2 border-t border-line py-2.5">
                <input
                  className="w-36 rounded-lg border border-line bg-white px-2.5 py-1.5 text-xs outline-none transition-colors placeholder:text-ink-3 focus:border-accent"
                  placeholder="按 IP 过滤"
                  value={draftIp}
                  onChange={(e) => setDraftIp(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && applyFilter()}
                />
                <input
                  className="w-28 rounded-lg border border-line bg-white px-2.5 py-1.5 text-xs outline-none transition-colors placeholder:text-ink-3 focus:border-accent"
                  placeholder="按代码过滤"
                  value={draftCode}
                  onChange={(e) => setDraftCode(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && applyFilter()}
                />
                <button
                  className="flex items-center gap-1 rounded-lg bg-accent px-2.5 py-1.5 text-xs text-white transition-opacity hover:opacity-90"
                  onClick={applyFilter}
                >
                  <Search size={12} />
                  查询
                </button>
                {(filter.ip || filter.code) && (
                  <button
                    className="text-xs text-ink-3 transition-colors hover:text-ink-2"
                    onClick={clearFilter}
                  >
                    清除
                  </button>
                )}
              </div>
              {!error && visits === null && (
                <div className="space-y-2 py-2">
                  {[0, 1, 2, 3].map((i) => (
                    <div key={i} className="skeleton h-7 rounded-lg" />
                  ))}
                </div>
              )}
              {!error && visits !== null && visits.length === 0 && (
                <div className="py-10 text-center text-sm text-ink-3">暂无记录</div>
              )}
              {!error && visits !== null && visits.length > 0 && (
                <table className="w-full min-w-[780px] border-collapse text-xs">
                  <thead>
                    <tr className="text-left text-ink-3">
                      {['时间', 'IP', '归属地', '请求', '代码', '状态', '耗时', '客户端'].map((h) => (
                        <th key={h} className="whitespace-nowrap pb-2 pr-4 font-medium">
                          {h}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {visits.map((v) => (
                      <tr key={v.id} className="border-t border-line">
                        <td className="tnum whitespace-nowrap py-2 pr-4 text-ink-2">{fmtTime(v.time)}</td>
                        <td className="tnum whitespace-nowrap py-2 pr-4">{v.ip}</td>
                        <td className="whitespace-nowrap py-2 pr-4 text-ink-2">{geoText(v)}</td>
                        <td
                          className="max-w-56 truncate py-2 pr-4"
                          title={`${v.method} ${v.path}${v.query ? '?' + v.query : ''}`}
                        >
                          <span className="text-ink-3">{v.method}</span> {v.path}
                          {v.query && <span className="text-ink-3">?{v.query}</span>}
                        </td>
                        <td className="tnum whitespace-nowrap py-2 pr-4">{v.code || '-'}</td>
                        <td className="whitespace-nowrap py-2 pr-4">
                          <StatusBadge s={v.status} />
                        </td>
                        <td className="tnum whitespace-nowrap py-2 pr-4 text-ink-2">{v.latency_ms}ms</td>
                        <td className="max-w-36 truncate py-2 text-ink-3" title={v.user_agent}>
                          {uaShort(v.user_agent)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}

// TokenGate 在服务端开启 --admin-token 时显示，输入后存入 localStorage。
function TokenGate({ onSubmit }: { onSubmit: (t: string) => void }) {
  const [draft, setDraft] = useState('')
  return (
    <div className="flex flex-col items-center gap-3 py-12">
      <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-accent-soft text-accent">
        <Lock size={18} />
      </div>
      <div className="text-sm font-medium">需要管理 token</div>
      <div className="text-xs text-ink-3">服务端已开启 --admin-token，请输入后查看</div>
      <form
        className="mt-1 flex items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          if (draft.trim()) onSubmit(draft.trim())
        }}
      >
        <input
          type="password"
          autoFocus
          className="w-56 rounded-lg border border-line bg-white px-3 py-2 text-xs outline-none transition-colors focus:border-accent"
          placeholder="admin token"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <button
          type="submit"
          className="rounded-lg bg-accent px-3 py-2 text-xs text-white transition-opacity hover:opacity-90"
        >
          确认
        </button>
      </form>
    </div>
  )
}

function StatsSection({ stats }: { stats: VisitStats }) {
  const cards: [string, number][] = [
    ['今日 PV', stats.today_pv],
    ['今日 UV', stats.today_uv],
    ['累计 PV', stats.total_pv],
    ['累计 UV', stats.total_uv],
  ]
  return (
    <div className="space-y-2 pb-1 pt-1">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {cards.map(([label, value]) => (
          <div key={label} className="rounded-xl border border-line bg-bg px-3 py-2.5">
            <div className="text-xs text-ink-3">{label}</div>
            <div className="tnum mt-0.5 text-lg font-semibold">{value}</div>
          </div>
        ))}
      </div>
      <div className="grid gap-2 sm:grid-cols-2">
        <TopList title="Top 归属地" items={stats.top_cities} />
        <TopList title="Top 搜索代码" items={stats.top_codes} />
      </div>
    </div>
  )
}

function TopList({ title, items }: { title: string; items: { name: string; count: number }[] }) {
  return (
    <div className="rounded-xl border border-line bg-bg px-3 py-2.5">
      <div className="text-xs text-ink-3">{title}</div>
      {items.length === 0 && <div className="mt-1.5 text-xs text-ink-3">暂无</div>}
      {items.slice(0, 5).map((it) => (
        <div key={it.name} className="mt-1 flex items-baseline justify-between text-xs">
          <span className="truncate">{it.name}</span>
          <span className="tnum ml-2 text-ink-3">{it.count}</span>
        </div>
      ))}
    </div>
  )
}

function StatusBadge({ s }: { s: number }) {
  const cls =
    s < 300
      ? 'bg-bear-soft text-bear'
      : s < 400
        ? 'bg-accent-soft text-accent'
        : s < 500
          ? 'bg-neutral-soft text-neutral'
          : 'bg-bull-soft text-bull'
  return <span className={`tnum rounded-md px-1.5 py-0.5 ${cls}`}>{s}</span>
}

function fmtTime(iso: string) {
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// geoText 拼接国家/省/市 (去重)，全空显示 "-"。
function geoText(v: Visit) {
  const parts = [v.country, v.province, v.city].filter(Boolean) as string[]
  return [...new Set(parts)].join(' ') || '-'
}

// uaShort 将原始 UA 解析为短格式 (存储仍保留原始串)。
function uaShort(ua?: string): string {
  if (!ua) return '-'
  if (/curl|wget/i.test(ua)) return 'curl'
  if (/bot\b|spider|crawl|slurp|Bytespider/i.test(ua)) return '爬虫'
  if (/python-requests|Go-http-client|Java\/|okhttp/i.test(ua)) return '脚本'
  let browser = ''
  if (/MicroMessenger/i.test(ua)) browser = '微信'
  else if (/Edg\//i.test(ua)) browser = 'Edge'
  else if (/OPR\//i.test(ua)) browser = 'Opera'
  else if (/Chrome\//i.test(ua)) browser = 'Chrome'
  else if (/Firefox\//i.test(ua)) browser = 'Firefox'
  else if (/Safari\//i.test(ua)) browser = 'Safari'
  let os = ''
  if (/Windows/i.test(ua)) os = 'Windows'
  else if (/iPhone|iPad/i.test(ua)) os = 'iOS'
  else if (/Android/i.test(ua)) os = 'Android'
  else if (/Mac OS X/i.test(ua)) os = 'macOS'
  else if (/HarmonyOS/i.test(ua)) os = 'HarmonyOS'
  else if (/Linux/i.test(ua)) os = 'Linux'
  return [browser, os].filter(Boolean).join(' · ') || '其他'
}
