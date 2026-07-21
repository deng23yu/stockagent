import { useState } from 'react'
import { Loader2, Search } from 'lucide-react'
import HotSearches from './HotSearches'

const EXAMPLES = [
  { code: '600519', name: '贵州茅台' },
  { code: '300750', name: '宁德时代' },
  { code: '000001', name: '平安银行' },
]

// parseCodes 解析逗号/空格分隔的代码输入 (去重，保持顺序)。
function parseCodes(raw: string): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const c of raw.split(/[,，\s]+/)) {
    if (c && !seen.has(c)) {
      seen.add(c)
      out.push(c)
    }
  }
  return out
}

export default function SearchBar({
  onSearch,
  onCompare,
  onDebate,
  onModeChange,
  loading,
}: {
  onSearch: (code: string, source: string) => void
  onCompare: (codes: string[], source: string) => void
  onDebate: (code: string, source: string) => void
  onModeChange: () => void
  loading: boolean
}) {
  const [mode, setMode] = useState<'single' | 'debate' | 'compare'>('single')
  const [code, setCode] = useState('')
  const [source, setSource] = useState<'eastmoney' | 'ths'>('ths')

  const codes = parseCodes(code)
  const valid =
    mode === 'compare'
      ? codes.length >= 2 && codes.length <= 4 && codes.every((c) => /^\d{6}$/.test(c))
      : /^\d{6}$/.test(code)

  const submit = () => {
    if (!valid || loading) return
    if (mode === 'single') onSearch(code, source)
    else if (mode === 'debate') onDebate(code, source)
    else onCompare(codes, source)
  }

  const buttonText = mode === 'single' ? '分析' : mode === 'debate' ? '开战' : '对比'

  return (
    <div className="fade-up [animation-delay:120ms]">
      <div className="mb-2.5 flex items-center gap-1 text-xs">
        {(
          [
            ['single', '单股分析'],
            ['debate', '多空辩论'],
            ['compare', '多股对比'],
          ] as const
        ).map(([v, label]) => (
          <button
            key={v}
            onClick={() => {
              if (v !== mode) {
                setMode(v)
                setCode('')
                onModeChange() // 切换模式时清掉上一模式的结果页
              }
            }}
            className={`rounded-full px-3 py-1.5 transition ${
              mode === v ? 'bg-accent-soft font-medium text-accent' : 'text-ink-3 hover:text-ink-2'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      <div className="shadow-pop flex items-center gap-2 rounded-2xl border border-line bg-white p-2 transition focus-within:border-accent/50 focus-within:ring-4 focus-within:ring-accent/10">
        <Search size={18} className="ml-3 shrink-0 text-ink-3" />
        <input
          value={code}
          onChange={(e) =>
            setCode(
              mode === 'compare'
                ? e.target.value.replace(/[^\d,，\s]/g, '').slice(0, 32)
                : e.target.value.replace(/\D/g, '').slice(0, 6),
            )
          }
          onKeyDown={(e) => e.key === 'Enter' && submit()}
          placeholder={
            mode === 'compare'
              ? '输入 2~4 个代码，逗号分隔，如 600519,000001'
              : mode === 'debate'
                ? '输入 6 位代码，看多空双方开辩'
                : '输入 6 位股票代码，如 600519'
          }
          inputMode="numeric"
          className="tnum h-11 w-full bg-transparent text-[15px] tracking-wide outline-none placeholder:text-ink-3"
        />
        <button
          onClick={submit}
          disabled={!valid || loading}
          className="h-11 shrink-0 rounded-xl bg-accent px-6 text-sm font-medium text-white transition hover:bg-accent/90 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {loading ? <Loader2 size={16} className="animate-spin" /> : buttonText}
        </button>
      </div>

      <div className="mt-3 flex items-center justify-between gap-3">
        <div className="flex items-center gap-1 rounded-full border border-line bg-white p-1 text-xs">
          {(
            [
              ['eastmoney', '东方财富'],
              ['ths', '同花顺'],
            ] as const
          ).map(([v, label]) => (
            <button
              key={v}
              onClick={() => setSource(v)}
              className={`rounded-full px-3 py-1.5 transition ${
                source === v ? 'bg-accent-soft font-medium text-accent' : 'text-ink-3 hover:text-ink-2'
              }`}
            >
              {label}
            </button>
          ))}
        </div>
        {mode === 'single' && (
          <div className="flex items-center gap-2 text-xs text-ink-3">
            <span className="hidden sm:inline">试试:</span>
            {EXAMPLES.map((e) => (
              <button
                key={e.code}
                onClick={() => onSearch(e.code, source)}
                disabled={loading}
                className="tnum rounded-full border border-line bg-white px-2.5 py-1 transition hover:border-accent/40 hover:text-accent disabled:opacity-40"
              >
                {e.name}
              </button>
            ))}
          </div>
        )}
      </div>

      {mode === 'single' && <HotSearches onPick={(c) => onSearch(c, source)} loading={loading} />}
    </div>
  )
}
