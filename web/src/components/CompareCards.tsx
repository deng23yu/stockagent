import { AlertTriangle } from 'lucide-react'
import { signalMeta } from '../lib/signal'
import type { CompareItem, Report } from '../lib/api'

// CompareCards 多股对比卡片网格 (2 列)，每股: 行情、四位分析师信号、综合结论。
export default function CompareCards({ items }: { items: CompareItem[] }) {
  return (
    <div className="fade-up grid gap-4 sm:grid-cols-2">
      {items.map((it) =>
        it.ok && it.report ? (
          <CompareCard key={it.code} report={it.report} />
        ) : (
          <div
            key={it.code}
            className="shadow-card flex items-start gap-2.5 rounded-2xl border border-bull/20 bg-bull-soft px-5 py-4 text-sm text-bull"
          >
            <AlertTriangle size={16} className="mt-0.5 shrink-0" />
            <span>
              <span className="tnum font-medium">{it.code}</span> {it.error ?? '分析失败'}
            </span>
          </div>
        ),
      )}
    </div>
  )
}

function CompareCard({ report }: { report: Report }) {
  const final = signalMeta(report.final.signal)
  const up = report.snapshot.change_pct >= 0
  return (
    <div className="shadow-card rounded-2xl border border-line bg-white px-5 py-4">
      <div className="flex items-baseline justify-between gap-2">
        <div className="min-w-0">
          <span className="truncate text-[15px] font-semibold tracking-tight">{report.name}</span>
          <span className="tnum ml-2 text-xs text-ink-3">{report.code}</span>
        </div>
        <div className="shrink-0 text-right">
          <span className="tnum text-[15px] font-semibold">{report.snapshot.price.toFixed(2)}</span>
          <span className={`tnum ml-1.5 text-xs ${up ? 'text-bull' : 'text-bear'}`}>
            {up ? '+' : ''}
            {report.snapshot.change_pct.toFixed(2)}%
          </span>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1.5">
        {report.results.map((r) => {
          const m = signalMeta(r.signal)
          return (
            <span key={r.agent} className="flex items-center gap-1 text-xs text-ink-2" title={r.reasoning}>
              <span className={`h-1.5 w-1.5 rounded-full ${m.bar}`} />
              {r.agent.replace('分析师', '')}
            </span>
          )
        })}
      </div>

      <div className="mt-3 flex items-center gap-2 border-t border-line pt-3">
        <span className={`rounded-md px-2 py-0.5 text-xs font-medium ${final.bg} ${final.text}`}>
          {final.label} {report.final.confidence}
        </span>
        <p className="line-clamp-2 text-xs leading-5 text-ink-2">{report.final.summary}</p>
      </div>
    </div>
  )
}
