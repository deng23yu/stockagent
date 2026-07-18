import { TrendingUp, TrendingDown, Minus } from 'lucide-react'

// A 股习惯: 红涨(看多) 绿跌(看空) 黄(中性)
export function signalMeta(signal: string) {
  switch (signal) {
    case 'bullish':
      return { label: '看多', text: 'text-bull', bg: 'bg-bull-soft', bar: 'bg-bull', Icon: TrendingUp }
    case 'bearish':
      return { label: '看空', text: 'text-bear', bg: 'bg-bear-soft', bar: 'bg-bear', Icon: TrendingDown }
    default:
      return { label: '中性', text: 'text-neutral', bg: 'bg-neutral-soft', bar: 'bg-neutral', Icon: Minus }
  }
}
