import type { AgentResult } from '../lib/api'
import { signalMeta } from '../lib/signal'

export default function SignalCards({
  results,
  className = '',
}: {
  results: AgentResult[]
  className?: string
}) {
  return (
    <section className={`grid gap-4 sm:grid-cols-2 ${className}`}>
      {results.map((r) => {
        const m = signalMeta(r.signal)
        return (
          <div key={r.agent} className="shadow-card rounded-2xl border border-line bg-white p-5">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">{r.agent}</span>
              <span
                className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  r.err ? 'bg-bg text-ink-3' : `${m.bg} ${m.text}`
                }`}
              >
                {r.err ? (
                  '失败'
                ) : (
                  <>
                    <m.Icon size={12} />
                    {m.label}
                  </>
                )}
              </span>
            </div>
            <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-bg">
              <div
                className={`h-full rounded-full transition-all duration-700 ${r.err ? 'bg-line' : m.bar}`}
                style={{ width: `${r.confidence}%` }}
              />
            </div>
            <div className="tnum mt-1 text-right text-xs text-ink-3">{r.confidence}</div>
            <p className="mt-2 text-[13px] leading-6 text-ink-2">{r.reasoning}</p>
          </div>
        )
      })}
    </section>
  )
}
