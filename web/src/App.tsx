import { useCallback, useRef, useState } from 'react'
import { AlertTriangle, ScrollText, Sparkles } from 'lucide-react'
import { analyzeStock, compareStocks, fetchDebate, type CompareItem, type Debate, type Report } from './lib/api'
import SearchBar from './components/SearchBar'
import MarketBar from './components/MarketBar'
import ActivityFeed from './components/ActivityFeed'
import NewsSection from './components/NewsSection'
import LoadingState from './components/LoadingState'
import ReportHeader from './components/ReportHeader'
import CapitalPanel from './components/CapitalPanel'
import SignalCards from './components/SignalCards'
import VerdictCard from './components/VerdictCard'
import CompareCards from './components/CompareCards'
import DebateView from './components/DebateView'
import VisitsPanel from './components/VisitsPanel'
import Footer from './components/Footer'

type Status =
  | { kind: 'idle' }
  | { kind: 'loading'; mode: 'analyze' | 'debate' | 'compare' }
  | { kind: 'success'; report: Report; cached: boolean }
  | { kind: 'compare'; items: CompareItem[] }
  | { kind: 'debate'; debate: Debate }
  | { kind: 'error'; message: string }

const DEBATE_STEPS = ['数据准备', '多方立论', '空方立论', '多方反驳', '空方反驳', '裁判裁决']

export default function App() {
  const [status, setStatus] = useState<Status>({ kind: 'idle' })
  const [showVisits, setShowVisits] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const onSearch = useCallback(async (code: string, source: string) => {
    abortRef.current?.abort()
    const ac = new AbortController()
    abortRef.current = ac
    setStatus({ kind: 'loading', mode: 'analyze' })
    try {
      const { report, cached } = await analyzeStock(code, source, ac.signal)
      setStatus({ kind: 'success', report, cached })
    } catch (e) {
      if ((e as Error).name === 'AbortError') return
      setStatus({ kind: 'error', message: (e as Error).message })
    }
  }, [])

  const onCompare = useCallback(async (codes: string[], source: string) => {
    abortRef.current?.abort()
    const ac = new AbortController()
    abortRef.current = ac
    setStatus({ kind: 'loading', mode: 'compare' })
    try {
      const items = await compareStocks(codes, source, ac.signal)
      setStatus({ kind: 'compare', items })
    } catch (e) {
      if ((e as Error).name === 'AbortError') return
      setStatus({ kind: 'error', message: (e as Error).message })
    }
  }, [])

  const onDebate = useCallback(async (code: string, source: string) => {
    abortRef.current?.abort()
    const ac = new AbortController()
    abortRef.current = ac
    setStatus({ kind: 'loading', mode: 'debate' })
    try {
      const debate = await fetchDebate(code, source, ac.signal)
      setStatus({ kind: 'debate', debate })
    } catch (e) {
      if ((e as Error).name === 'AbortError') return
      setStatus({ kind: 'error', message: (e as Error).message })
    }
  }, [])

  // 切换分析模式: 中止在途请求并回到初始页 (避免残留上一模式的结果)
  const onModeChange = useCallback(() => {
    abortRef.current?.abort()
    setStatus({ kind: 'idle' })
  }, [])

  return (
    <div className="relative min-h-screen">
      {/* 顶部蓝紫光晕 */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-0 h-72 bg-[radial-gradient(60%_100%_at_50%_0%,rgba(91,91,214,0.10),transparent)]"
      />
      <div className="relative mx-auto max-w-3xl px-5 pb-24">
        <header className="fade-up flex items-center gap-2.5 py-7">
          <div className="shadow-pop flex h-9 w-9 items-center justify-center rounded-xl bg-accent text-white">
            <Sparkles size={18} />
          </div>
          <div>
            <div className="text-[15px] font-semibold tracking-tight">stockagent</div>
            <div className="text-xs text-ink-3">A 股 AI 投研多智能体</div>
          </div>
          <button
            className="ml-auto flex items-center gap-1 text-xs text-ink-3 transition-colors hover:text-ink-2"
            onClick={() => setShowVisits(true)}
          >
            <ScrollText size={13} />
            访客记录
          </button>
          <a
            className="ml-4 text-xs text-ink-3 transition-colors hover:text-ink-2"
            href="https://github.com/deng23yu/stockagent"
            target="_blank"
            rel="noreferrer"
          >
            GitHub
          </a>
        </header>

        <MarketBar />

        <section className="fade-up mb-8 mt-10 text-center [animation-delay:60ms]">
          <h1 className="text-4xl font-semibold leading-tight tracking-tight sm:text-[44px]">
            一条命令，看懂一只股票
          </h1>
          <p className="mt-4 text-[15px] leading-7 text-ink-2">
            四位 AI 分析师并行研判，组合经理汇总成一份中文投研报告。
            <br className="hidden sm:block" />
            数据免费，模型自备，仅供学习研究。
          </p>
        </section>

        <SearchBar
          onSearch={onSearch}
          onCompare={onCompare}
          onDebate={onDebate}
          onModeChange={onModeChange}
          loading={status.kind === 'loading'}
        />
        <ActivityFeed />

        <main className="mt-10 space-y-4">
          {status.kind === 'idle' && (
            <>
              <IdleHint />
              <NewsSection />
            </>
          )}
          {status.kind === 'loading' && (
            <LoadingState steps={status.mode === 'debate' ? DEBATE_STEPS : undefined} />
          )}
          {status.kind === 'error' && (
            <div className="fade-up flex items-start gap-2.5 rounded-2xl border border-bull/20 bg-bull-soft px-5 py-4 text-sm text-bull">
              <AlertTriangle size={16} className="mt-0.5 shrink-0" />
              <span>{status.message}</span>
            </div>
          )}
          {status.kind === 'success' && (
            <>
              <ReportHeader report={status.report} cached={status.cached} className="fade-up" />
              <CapitalPanel code={status.report.code} />
              <SignalCards results={status.report.results} className="fade-up [animation-delay:120ms]" />
              <VerdictCard final={status.report.final} className="fade-up [animation-delay:240ms]" />
            </>
          )}
          {status.kind === 'compare' && <CompareCards items={status.items} />}
          {status.kind === 'debate' && (
            <DebateView key={status.debate.code + status.debate.generated_at} debate={status.debate} />
          )}
        </main>

        <Footer report={status.kind === 'success' ? status.report : null} />
      </div>
      {showVisits && <VisitsPanel onClose={() => setShowVisits(false)} />}
    </div>
  )
}

function IdleHint() {
  const feats = [
    ['免 key 行情', '东方财富 / 同花顺双数据源'],
    ['多智能体', '技术面 · 基本面 · 消息面 · 风控'],
    ['结构化结论', '信号 + 置信度 + 关键要点'],
  ] as const
  return (
    <div className="fade-up grid gap-3 [animation-delay:120ms] sm:grid-cols-3">
      {feats.map(([t, d]) => (
        <div key={t} className="shadow-card rounded-2xl border border-line bg-white px-5 py-4">
          <div className="text-sm font-medium">{t}</div>
          <div className="mt-1 text-xs leading-5 text-ink-3">{d}</div>
        </div>
      ))}
    </div>
  )
}
