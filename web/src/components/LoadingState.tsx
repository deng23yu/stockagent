import { useEffect, useState } from 'react'
import { CheckCircle2, Loader2 } from 'lucide-react'

const STEPS = ['技术面分析师', '基本面分析师', '消息面分析师', '风控官', '组合经理汇总']

export default function LoadingState() {
  const [active, setActive] = useState(0)
  useEffect(() => {
    const t = setInterval(() => setActive((i) => Math.min(i + 1, STEPS.length - 1)), 6000)
    return () => clearInterval(t)
  }, [])

  return (
    <div className="fade-up shadow-card rounded-2xl border border-line bg-white p-6">
      <div className="space-y-3">
        {STEPS.map((name, i) => (
          <div key={name} className="flex items-center gap-3 text-sm">
            {i < active ? (
              <CheckCircle2 size={16} className="shrink-0 text-bear" />
            ) : i === active ? (
              <Loader2 size={16} className="shrink-0 animate-spin text-accent" />
            ) : (
              <span className="h-4 w-4 shrink-0 rounded-full border border-line" />
            )}
            <span className={i <= active ? 'text-ink' : 'text-ink-3'}>{name}</span>
            {i === active && <span className="pulse-soft text-xs text-ink-3">分析中…</span>}
          </div>
        ))}
      </div>
      <div className="mt-6 space-y-2.5">
        <div className="skeleton h-4 w-3/4 rounded" />
        <div className="skeleton h-4 w-1/2 rounded" />
      </div>
      <p className="mt-4 text-xs text-ink-3">真实分析约需 20-40 秒，请稍候</p>
    </div>
  )
}
