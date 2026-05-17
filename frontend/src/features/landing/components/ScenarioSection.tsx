import { PlayCircle, Terminal } from 'lucide-react'
import { scenarioCommands } from '@/features/landing/landingData'

export function ScenarioSection() {
  return (
    <section id="scenario-lab" className="border-b border-slate-200 bg-white py-16 sm:py-20">
      <div className="mx-auto grid max-w-[1440px] gap-8 px-4 sm:px-6 lg:grid-cols-[0.82fr_1fr] lg:items-center lg:px-8">
        <div>
          <h2 className="max-w-xl text-3xl font-bold leading-tight text-slate-950 sm:text-5xl">
            Try it with realistic failure scenarios.
          </h2>
          <p className="mt-4 max-w-xl text-base leading-7 text-slate-600">
            The demo stack includes scripts for API outages, slow responses, MySQL load, RCA generation, and recovery. They exercise the same code paths used by the product.
          </p>
          <div className="mt-6 flex flex-col gap-3 sm:flex-row">
            <a
              href="#top"
              className="inline-flex items-center justify-center gap-2 rounded-[8px] bg-slate-950 px-5 py-3 text-sm font-semibold text-white shadow-xl shadow-slate-950/20 transition hover:bg-slate-800"
            >
              Run the demo
              <PlayCircle className="h-4 w-4" />
            </a>
            <a
              href="/help/getting-started"
              className="inline-flex items-center justify-center rounded-[8px] border border-slate-300 bg-white px-5 py-3 text-sm font-semibold text-slate-800 shadow-sm transition hover:border-slate-400 hover:bg-slate-50"
            >
              Read the guides
            </a>
          </div>
        </div>

        <div className="overflow-hidden rounded-[8px] border border-slate-900 bg-slate-950 shadow-2xl shadow-slate-950/20">
          <div className="flex items-center gap-2 border-b border-white/10 px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-400">
            <Terminal className="h-4 w-4" />
            Scenario commands
          </div>
          <pre className="overflow-x-auto p-5 text-sm leading-7 text-slate-200">
            <code className="whitespace-pre-wrap break-words">{scenarioCommands.join('\n')}</code>
          </pre>
        </div>
      </div>
    </section>
  )
}
