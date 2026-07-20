import { useEffect, useState } from 'react'
import { Flame } from 'lucide-react'
import { fetchHotSearches, type HotSearch } from '../lib/api'

// HotSearches 热门搜索榜: 展示最近 7 天被搜索最多的代码，点击直接分析。
export default function HotSearches({
  onPick,
  loading,
}: {
  onPick: (code: string) => void
  loading: boolean
}) {
  const [items, setItems] = useState<HotSearch[]>([])

  useEffect(() => {
    fetchHotSearches()
      .then(setItems)
      .catch(() => {})
  }, [])

  if (items.length === 0) return null

  return (
    <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-ink-3">
      <Flame size={13} className="text-bull" />
      <span>热门:</span>
      {items.map((it) => (
        <button
          key={it.name}
          onClick={() => onPick(it.name)}
          disabled={loading}
          className="tnum rounded-full border border-line bg-white px-2.5 py-1 transition hover:border-accent/40 hover:text-accent disabled:opacity-40"
        >
          {it.name}
          <span className="ml-1 text-ink-3">×{it.count}</span>
        </button>
      ))}
    </div>
  )
}
