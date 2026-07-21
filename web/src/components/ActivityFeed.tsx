import { useEffect, useState } from 'react'
import { fetchActivity, type Activity } from '../lib/api'

// ActivityFeed 全站实时动态: 脉冲红点 + 单条轮播 (5s 切换)，60s 拉新，失败静默。
export default function ActivityFeed() {
  const [items, setItems] = useState<Activity[]>([])
  const [idx, setIdx] = useState(0)

  useEffect(() => {
    let alive = true
    const load = () =>
      fetchActivity()
        .then((d) => alive && setItems(d))
        .catch(() => {})
    load()
    const timer = setInterval(load, 60_000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [])

  useEffect(() => {
    if (items.length < 2) return
    const t = setInterval(() => setIdx((i) => i + 1), 5_000)
    return () => clearInterval(t)
  }, [items.length])

  if (items.length === 0) return null
  const it = items[idx % items.length]

  return (
    <div className="mt-3 flex items-center gap-2 text-xs text-ink-3">
      <span className="relative flex h-2 w-2 shrink-0">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-bull opacity-60" />
        <span className="relative inline-flex h-2 w-2 rounded-full bg-bull" />
      </span>
      <span key={`${idx}-${it.time}`} className="fade-up truncate">
        {timeAgo(it.time)} · {it.city}股友
        {it.action === 'compare' ? ` 对比了 ${it.codes.join(' / ')}` : ` 分析了 ${it.codes[0]}`}
      </span>
    </div>
  )
}

function timeAgo(iso: string) {
  const s = Math.max(1, Math.floor((Date.now() - new Date(iso).getTime()) / 1000))
  if (s < 60) return '刚刚'
  const m = Math.floor(s / 60)
  if (m < 60) return `${m} 分钟前`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} 小时前`
  return `${Math.floor(h / 24)} 天前`
}
