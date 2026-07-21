import { useEffect, useState } from 'react'
import { FastForward, Gavel, Trophy } from 'lucide-react'
import type { Debate, DebateTurn } from '../lib/api'

// DebateView 多空辩论实录: 发言气泡逐条揭示 (多方居左红 / 空方居右绿)，
// 全部发言后展示裁判裁决卡。
export default function DebateView({ debate }: { debate: Debate }) {
  const total = debate.turns.length
  const [shown, setShown] = useState(1)
  const done = shown > total

  useEffect(() => {
    if (shown > total) return
    const t = setTimeout(() => setShown((s) => s + 1), 1400)
    return () => clearTimeout(t)
  }, [shown, total])

  return (
    <div className="space-y-3">
      <div className="fade-up flex items-center justify-between">
        <div className="text-sm font-semibold tracking-tight">
          {debate.name} <span className="tnum text-xs font-normal text-ink-3">{debate.code} · 多空辩论</span>
        </div>
        {!done && (
          <button
            className="flex items-center gap-1 text-xs text-ink-3 transition-colors hover:text-ink-2"
            onClick={() => setShown(total + 1)}
          >
            <FastForward size={12} />
            快进
          </button>
        )}
      </div>

      {debate.turns.slice(0, Math.min(shown, total)).map((t, i) => (
        <Bubble key={i} turn={t} />
      ))}

      {!done && (
        <div className={`fade-up flex ${debate.turns[Math.min(shown, total)]?.role === 'bear' ? 'justify-end' : 'justify-start'}`}>
          <div className="shadow-card rounded-2xl border border-line bg-white px-4 py-3">
            <span className="pulse-soft text-xs text-ink-3">对方思考中…</span>
          </div>
        </div>
      )}

      {done && <VerdictCard debate={debate} />}
    </div>
  )
}

function Bubble({ turn }: { turn: DebateTurn }) {
  const bull = turn.role === 'bull'
  return (
    <div className={`fade-up flex ${bull ? 'justify-start' : 'justify-end'}`}>
      <div
        className={`max-w-[85%] rounded-2xl border px-4 py-3 ${
          bull ? 'border-bull/15 bg-bull-soft/50' : 'border-bear/15 bg-bear-soft/50'
        }`}
      >
        <div className={`mb-1 text-[11px] font-medium ${bull ? 'text-bull' : 'text-bear'}`}>
          {bull ? '多方辩手' : '空方辩手'} · 第 {turn.round} 轮
        </div>
        <p className="text-sm leading-6">{turn.content}</p>
      </div>
    </div>
  )
}

function VerdictCard({ debate }: { debate: Debate }) {
  const v = debate.verdict
  const title = v.winner === 'bull' ? '多方获胜' : v.winner === 'bear' ? '空方获胜' : '平局'
  const color = v.winner === 'bull' ? 'text-bull' : v.winner === 'bear' ? 'text-bear' : 'text-neutral'
  const totalScore = v.bull_score + v.bear_score || 1
  return (
    <div className="fade-up shadow-card rounded-2xl border border-line bg-white px-5 py-4">
      <div className="flex items-center gap-2">
        <Gavel size={15} className="text-ink-3" />
        <span className="text-xs text-ink-3">裁判裁决</span>
        <span className={`flex items-center gap-1 text-[15px] font-semibold ${color}`}>
          <Trophy size={14} />
          {title}
        </span>
      </div>
      <div className="mt-3 space-y-2">
        <ScoreBar label="多方" score={v.bull_score} max={totalScore} bar="bg-bull" text="text-bull" />
        <ScoreBar label="空方" score={v.bear_score} max={totalScore} bar="bg-bear" text="text-bear" />
      </div>
      <p className="mt-3 text-xs leading-5 text-ink-2">{v.reasoning}</p>
    </div>
  )
}

function ScoreBar({
  label,
  score,
  max,
  bar,
  text,
}: {
  label: string
  score: number
  max: number
  bar: string
  text: string
}) {
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="w-8 text-ink-3">{label}</span>
      <div className="h-1.5 flex-1 rounded-full bg-black/5">
        <div className={`h-full rounded-full ${bar}`} style={{ width: `${(score / max) * 100}%` }} />
      </div>
      <span className={`tnum w-8 text-right font-medium ${text}`}>{score}</span>
    </div>
  )
}
