import { useCallback, useEffect, useState } from 'react'
import { ChevronDown, Newspaper, RefreshCw } from 'lucide-react'
import { fetchNews, type NewsItem } from '../lib/api'

// NewsSection 主页宏观快讯: 标题列表点击展开全文 (手风琴)，手动刷新，每 5 分钟自刷。
export default function NewsSection() {
  const [items, setItems] = useState<NewsItem[] | null>(null)
  const [openId, setOpenId] = useState<number | null>(null)
  const [refreshing, setRefreshing] = useState(false)

  const load = useCallback(() => {
    setRefreshing(true)
    fetchNews()
      .then((d) => setItems(d.items))
      .catch(() => {})
      .finally(() => setRefreshing(false))
  }, [])

  useEffect(() => {
    load()
    const t = setInterval(load, 5 * 60_000)
    return () => clearInterval(t)
  }, [load])

  if (items === null) {
    return (
      <div className="fade-up shadow-card rounded-2xl border border-line bg-white px-5 py-4 [animation-delay:180ms]">
        <div className="skeleton h-5 w-32 rounded" />
        <div className="mt-3 space-y-2.5">
          {[0, 1, 2].map((i) => (
            <div key={i} className="skeleton h-4 w-full rounded" />
          ))}
        </div>
      </div>
    )
  }
  if (items.length === 0) return null

  return (
    <section className="fade-up shadow-card rounded-2xl border border-line bg-white px-5 py-4 [animation-delay:180ms]">
      <div className="mb-2 flex items-center gap-2">
        <Newspaper size={14} className="text-accent" />
        <span className="text-sm font-semibold tracking-tight">宏观快讯</span>
        <button
          className="ml-auto rounded-lg p-1.5 text-ink-3 transition-colors hover:bg-accent-soft hover:text-accent disabled:opacity-40"
          onClick={load}
          disabled={refreshing}
          title="刷新"
        >
          <RefreshCw size={13} className={refreshing ? 'animate-spin' : ''} />
        </button>
      </div>
      <div className="divide-y divide-line">
        {items.slice(0, 8).map((it) => {
          const open = openId === it.id
          return (
            <div key={it.id}>
              <button
                className="flex w-full items-center gap-2 py-2.5 text-left transition-colors hover:text-accent"
                onClick={() => setOpenId(open ? null : it.id)}
              >
                <span className="shrink-0 text-[11px] text-ink-3">{timeAgo(it.time)}</span>
                <span className={`min-w-0 flex-1 truncate text-sm ${open ? 'font-medium text-accent' : ''}`}>
                  {it.title}
                </span>
                <ChevronDown
                  size={14}
                  className={`shrink-0 text-ink-3 transition-transform ${open ? 'rotate-180' : ''}`}
                />
              </button>
              {open && (
                <p className="fade-up pb-3 text-[13px] leading-6 text-ink-2">{it.content}</p>
              )}
            </div>
          )
        })}
      </div>
    </section>
  )
}

function timeAgo(timeStr: string) {
  const s = Math.max(1, Math.floor((Date.now() - new Date(timeStr.replace(' ', 'T')).getTime()) / 1000))
  if (s < 3600) return `${Math.max(1, Math.floor(s / 60))}分钟前`
  if (s < 86400) return `${Math.floor(s / 3600)}小时前`
  return `${Math.floor(s / 86400)}天前`
}
