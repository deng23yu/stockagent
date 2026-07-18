import type { FinalVerdict } from '../lib/api'
import { signalMeta } from '../lib/signal'

export default function VerdictCard({
  final,
  className = '',
}: {
  final: FinalVerdict
  className?: string
}) {
  const m = signalMeta(final.signal)
  return (
    <section className={`shadow-pop rounded-2xl border border-accent/20 bg-white p-6 ${className}`}>
      <div className="text-sm font-medium text-accent">综合结论</div>
      <div className="mt-2 flex items-baseline gap-3">
        <span className={`text-3xl font-semibold tracking-tight ${m.text}`}>{m.label}</span>
        <span className="tnum text-sm text-ink-3">置信度 {final.confidence}</span>
      </div>
      <p className="mt-4 text-[15px] leading-7 text-ink">{final.summary}</p>
      {final.key_points?.length > 0 && (
        <ol className="mt-4 space-y-2">
          {final.key_points.map((p, i) => (
            <li key={i} className="flex gap-3 text-sm leading-6 text-ink-2">
              <span className="tnum mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-accent-soft text-xs text-accent">
                {i + 1}
              </span>
              {p}
            </li>
          ))}
        </ol>
      )}
      {final.degraded && (
        <p className="mt-3 text-xs text-ink-3">LLM 组合经理不可用，本结论为本地加权聚合</p>
      )}
    </section>
  )
}
