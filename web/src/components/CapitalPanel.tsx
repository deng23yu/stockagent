import { useEffect, useState } from 'react'
import { fetchCapital, type CapitalData, type FundFlowDay } from '../lib/api'

// CapitalPanel 个股资金面板: 两融 / 主力净流入 / 近5日资金流柱图 / 沪深港通持股。
export default function CapitalPanel({ code }: { code: string }) {
  const [data, setData] = useState<CapitalData | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let alive = true
    setData(null)
    setFailed(false)
    fetchCapital(code)
      .then((d) => alive && setData(d))
      .catch(() => alive && setFailed(true))
    return () => {
      alive = false
    }
  }, [code])

  if (failed) return null
  if (!data) {
    return (
      <div className="shadow-card rounded-2xl border border-line bg-white px-5 py-4">
        <div className="skeleton h-4 w-20 rounded" />
        <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="skeleton h-12 rounded-lg" />
          ))}
        </div>
      </div>
    )
  }

  const flows = data.fund_flow ?? []
  const today = flows.length > 0 ? flows[flows.length - 1] : null
  const sum5 = flows.reduce((a, b) => a + b.main, 0)

  return (
    <div className="fade-up shadow-card rounded-2xl border border-line bg-white px-5 py-4 [animation-delay:60ms]">
      <div className="mb-3 text-xs font-medium text-ink-3">资金面板</div>
      <div className={`grid grid-cols-2 gap-3 ${flows.length >= 2 ? 'sm:grid-cols-4' : 'sm:grid-cols-3'}`}>
        <Stat label="融资余额" value={fmtYi(data.margin?.rzye)} sub={data.margin?.date.slice(5)} />
        <Stat label="融券余额" value={fmtYi(data.margin?.rqye)} />
        <Stat label="今日主力净流入" value={fmtSignedYi(today?.main)} signed={today?.main} />
        {flows.length >= 2 && (
          <Stat label={`近${flows.length}日主力净流入`} value={fmtSignedYi(sum5)} signed={sum5} />
        )}
      </div>

      {flows.length > 1 && <FlowBars flows={flows} />}

      {data.northbound && (
        <div className="mt-3 border-t border-line pt-3 text-xs text-ink-2">
          沪深港通持股
          <span className="tnum ml-1.5 font-semibold">{data.northbound.shares_ratio.toFixed(2)}%</span>
          <span className="ml-2 text-ink-3">
            ({fmtWanGu(data.northbound.shares)} · {data.northbound.date.slice(5)} 披露)
          </span>
        </div>
      )}
    </div>
  )
}

function Stat({ label, value, sub, signed }: { label: string; value: string; sub?: string; signed?: number }) {
  const color = signed === undefined ? '' : signed >= 0 ? 'text-bull' : 'text-bear'
  return (
    <div className="rounded-xl border border-line bg-bg px-3 py-2.5">
      <div className="text-[11px] text-ink-3">{label}</div>
      <div className={`tnum mt-0.5 text-[15px] font-semibold ${color}`}>{value}</div>
      {sub && <div className="mt-0.5 text-[10px] text-ink-3">{sub}</div>}
    </div>
  )
}

// FlowBars 近 5 日主力净流入柱图 (红进绿出)。
function FlowBars({ flows }: { flows: FundFlowDay[] }) {
  const max = Math.max(...flows.map((f) => Math.abs(f.main)), 1)
  return (
    <div className="mt-4 flex items-end justify-between gap-2">
      {flows.map((f) => (
        <div key={f.date} className="flex flex-1 flex-col items-center gap-1">
          <span className={`tnum text-[10px] ${f.main >= 0 ? 'text-bull' : 'text-bear'}`}>
            {(f.main / 1e8).toFixed(1)}
          </span>
          <div className="flex h-14 w-full items-center justify-center">
            <div
              className={`w-5 rounded-sm ${f.main >= 0 ? 'bg-bull' : 'bg-bear'}`}
              style={{ height: `${Math.max(8, (Math.abs(f.main) / max) * 100)}%` }}
            />
          </div>
          <span className="text-[10px] text-ink-3">{f.date.slice(5)}</span>
        </div>
      ))}
    </div>
  )
}

function fmtYi(v?: number) {
  if (v === undefined) return '—'
  return (v / 1e8).toFixed(2) + ' 亿'
}

function fmtSignedYi(v?: number) {
  if (v === undefined) return '—'
  return (v >= 0 ? '+' : '-') + (Math.abs(v) / 1e8).toFixed(2) + ' 亿'
}

function fmtWanGu(v: number) {
  return (v / 1e4).toFixed(0) + ' 万股'
}
