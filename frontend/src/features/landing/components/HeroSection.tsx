import { Link } from 'react-router-dom'
import { ArrowRight, Github, Terminal } from 'lucide-react'
import { proofItems, quickStartCommand, repoUrl } from '@/features/landing/landingData'

export function HeroSection() {
  return (
    <section id="top" className="landing-grid border-b border-slate-200 bg-slate-50">
      <div className="mx-auto grid max-w-[1440px] grid-cols-1 gap-10 px-4 py-12 sm:px-6 sm:py-16 lg:grid-cols-[minmax(0,0.88fr)_minmax(420px,0.62fr)] lg:px-8 lg:py-20">
        <div className="min-w-0 flex flex-col justify-center">
          <h1 className="max-w-4xl text-4xl font-bold leading-[1.04] tracking-normal text-slate-950 sm:text-5xl lg:text-6xl">
            Detect, heal, and explain — before you open the dashboard.
          </h1>
          <p className="mt-6 max-w-2xl text-lg leading-8 text-slate-600">
            Open-source uptime checks, log ingestion, incident response, auto-remediation, status pages, and MySQL triage in one self-hosted service. Bring your own AI key for root-cause analysis or run without it.
          </p>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-500">
            MongoDB-backed persistence. Built for small teams, not distributed tracing or high-volume log aggregation.
          </p>

          <div className="mt-8 flex flex-col gap-3 sm:flex-row">
            <Link
              to="/login"
              className="inline-flex items-center justify-center gap-2 rounded-[8px] bg-slate-950 px-5 py-3 text-sm font-semibold text-white shadow-xl shadow-slate-950/20 transition hover:bg-slate-800"
            >
              Sign in
              <ArrowRight className="h-4 w-4" />
            </Link>
            <a
              href={repoUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center justify-center gap-2 rounded-[8px] border border-slate-300 bg-white px-5 py-3 text-sm font-semibold text-slate-800 shadow-sm transition hover:border-slate-400 hover:bg-slate-50"
            >
              <Github className="h-4 w-4" />
              View on GitHub
            </a>
          </div>

          <div className="mt-8 hidden max-w-2xl grid-cols-2 gap-3 sm:grid sm:grid-cols-4">
            {proofItems.map((item) => (
              <div key={item.label} className="rounded-[8px] border border-slate-200 bg-white/80 p-3 shadow-sm backdrop-blur">
                <div className="text-sm font-bold leading-5 text-slate-950 sm:text-base">{item.value}</div>
                <div className="mt-0.5 text-xs font-medium text-slate-500">{item.label}</div>
              </div>
            ))}
          </div>
        </div>

        <div className="hidden min-w-0 items-center sm:flex">
          <div className="w-full overflow-hidden rounded-[8px] border border-slate-900 bg-slate-950 shadow-2xl shadow-slate-950/20">
            <div className="flex items-center justify-between border-b border-white/10 px-4 py-3">
              <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-slate-400">
                <Terminal className="h-4 w-4" />
                Quick start
              </div>
              <span className="rounded-[6px] border border-emerald-300/20 bg-emerald-300/10 px-2 py-1 text-[11px] font-semibold text-emerald-200">
                Docker Compose
              </span>
            </div>
            <pre className="overflow-x-auto p-5 text-xs leading-6 text-slate-200 sm:text-sm">
              <code className="whitespace-pre-wrap break-words">{quickStartCommand}</code>
            </pre>
            <div className="border-t border-white/10 bg-white/[0.04] px-5 py-4 text-sm leading-6 text-slate-300">
              The bundled stack includes MongoDB, MySQL, Redis, two SSH targets, log emission, and a local AI provider for RCA testing.
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
