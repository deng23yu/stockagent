import type { Report } from '../lib/api'

export default function Footer({ report }: { report: Report | null }) {
  return (
    <footer className="fade-up mt-14 space-y-2 text-center text-xs leading-5 text-ink-3">
      <p>
        数据: 东方财富 / 同花顺 · 公告: 东方财富
        {report && ` · 模型: ${report.model}`}
      </p>
      <p className="mx-auto max-w-xl">
        风险提示: 本报告由 AI 自动生成，仅供学习与技术研究，不构成任何投资建议。股市有风险，投资需谨慎。
      </p>
    </footer>
  )
}
