import type { Report } from '../lib/api'
import { signalMeta } from '../lib/signal'

const fmtCap = (v: number) =>
  v >= 1e12 ? `${(v / 1e12).toFixed(2)} 万亿` : v >= 1e8 ? `${(v / 1e8).toFixed(0)} 亿` : `${v}`

export default function ReportHeader({
  report,
  cached,
  className = '',
}: {
  report: Report
  cached: boolean
  className?: string
}) {
  const s = report.snapshot
  const m = signalMeta(report.final.signal)
  const up = s.change_pct >= 0
  return (
    <section className={`shadow-card rounded-2xl border border-line bg-white p-6 ${className}`}>
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-xl font-semibold tracking-tight">{report.name}</h2>
        <span className="tnum rounded-md bg-bg px-2 py-0.5 text-xs text-ink-3">{report.code}</span>
        {cached && <span className="rounded-md bg-accent-soft px-2 py-0.5 text-xs text-accent">缓存结果</span>}
        <span
          className={`ml-auto inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-medium ${m.bg} ${m.text}`}
        >
          <m.Icon size={14} /> {m.label} · {report.final.confidence}
        </span>
      </div>

      <div className="mt-5 flex flex-wrap items-end gap-x-10 gap-y-4">
        <div>
          <div className={`tnum text-5xl font-semibold tracking-tight ${up ? 'text-bull' : 'text-bear'}`}>
            {s.price.toFixed(2)}
          </div>
          <div className={`tnum mt-1 text-sm ${up ? 'text-bull' : 'text-bear'}`}>
            {up ? '+' : ''}
            {s.change_pct.toFixed(2)}%
          </div>
        </div>
        <div className="grid grid-cols-3 gap-x-8 gap-y-2 text-sm">
          {[
            ['PE(动)', s.pe > 0 ? s.pe.toFixed(2) : '亏损'],
            ['PB', s.pb > 0 ? s.pb.toFixed(2) : '—'],
            ['总市值', fmtCap(s.total_market_cap)],
          ].map(([k, v]) => (
            <div key={k}>
              <div className="text-xs text-ink-3">{k}</div>
              <div className="tnum mt-0.5 font-medium">{v}</div>
            </div>
          ))}
        </div>
      </div>

      <div className="mt-5 text-xs text-ink-3">
        生成于 {new Date(report.generated_at).toLocaleString('zh-CN')} · 模型 {report.model}
      </div>
    </section>
  )
}
