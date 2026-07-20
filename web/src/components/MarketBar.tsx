import { useEffect, useState } from 'react'
import { TrendingDown, TrendingUp } from 'lucide-react'
import { fetchMarket, type IndexQuote, type MarketData } from '../lib/api'

// MarketBar 市场概览: 三大指数卡片 (大号点位 + 涨跌 pill + 30 日走势图)，
// 每 60s 自刷，失败静默隐藏。A 股习惯: 红涨绿跌。
export default function MarketBar() {
  const [data, setData] = useState<MarketData | null>(null)

  useEffect(() => {
    let alive = true
    const load = () =>
      fetchMarket()
        .then((d) => alive && setData(d))
        .catch(() => {})
    load()
    const timer = setInterval(load, 60_000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [])

  if (!data || data.indices.length === 0) return null

  return (
    <div className="fade-up grid grid-cols-3 gap-2.5 sm:gap-3">
      {data.indices.map((ix) => (
        <IndexCard key={ix.code} ix={ix} />
      ))}
    </div>
  )
}

function IndexCard({ ix }: { ix: IndexQuote }) {
  const up = ix.change_pct >= 0
  const color = up ? 'text-bull' : 'text-bear'
  const Icon = up ? TrendingUp : TrendingDown
  return (
    <div className="shadow-card rounded-2xl border border-line bg-white px-3.5 py-3 sm:px-4">
      <div className="flex items-center justify-between gap-1">
        <span className="truncate text-xs text-ink-3">{ix.name}</span>
        <span
          className={`tnum flex shrink-0 items-center gap-0.5 rounded-md px-1.5 py-0.5 text-[11px] font-medium ${
            up ? 'bg-bull-soft text-bull' : 'bg-bear-soft text-bear'
          }`}
        >
          <Icon size={11} />
          {up ? '+' : ''}
          {ix.change_pct.toFixed(2)}%
        </span>
      </div>
      <div className={`tnum mt-1 text-lg font-semibold tracking-tight sm:text-xl ${color}`}>
        {ix.price.toFixed(2)}
      </div>
      {ix.closes && ix.closes.length > 1 && <Sparkline closes={ix.closes} up={up} />}
    </div>
  )
}

// Sparkline 近 30 日走势: 折线 + 渐变面积 + 末端圆点 (纯 SVG，无图表库)。
function Sparkline({ closes, up }: { closes: number[]; up: boolean }) {
  const w = 100
  const h = 32
  const pad = 2.5
  const min = Math.min(...closes)
  const max = Math.max(...closes)
  const span = max - min || 1
  const pts = closes.map(
    (c, i): [number, number] => [
      (i / (closes.length - 1)) * w,
      h - pad - ((c - min) / span) * (h - pad * 2),
    ],
  )
  const line = pts.map((p) => `${p[0].toFixed(1)},${p[1].toFixed(1)}`).join(' ')
  const color = up ? '#e5484d' : '#30a46c'
  const gid = `spark-${up ? 'up' : 'down'}`
  const [lastX, lastY] = pts[pts.length - 1]
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="mt-1.5 h-8 w-full" preserveAspectRatio="none" aria-hidden>
      <defs>
        <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity={0.22} />
          <stop offset="100%" stopColor={color} stopOpacity={0.02} />
        </linearGradient>
      </defs>
      <polygon points={`0,${h} ${line} ${w},${h}`} fill={`url(#${gid})`} />
      <polyline
        points={line}
        fill="none"
        stroke={color}
        strokeWidth={1.5}
        strokeLinejoin="round"
        strokeLinecap="round"
        vectorEffect="non-scaling-stroke"
      />
      <circle cx={lastX} cy={lastY} r={1.8} fill={color} />
    </svg>
  )
}
