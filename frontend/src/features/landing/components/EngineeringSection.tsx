import { engineeringItems, productionCommand } from '@/features/landing/landingData'

export function EngineeringSection() {
  return (
    <section id="engineering" className="border-b border-slate-200 bg-white py-16 sm:py-20">
      <div className="mx-auto max-w-[1440px] px-4 sm:px-6 lg:px-8">
        <div className="grid gap-8 lg:grid-cols-[0.72fr_1fr] lg:items-start">
          <div className="min-w-0">
            <h2 className="max-w-2xl text-3xl font-bold leading-tight text-slate-950 sm:text-5xl">
              Written in Go, because monitoring should not be the thing that crashes.
            </h2>
            <p className="mt-4 max-w-2xl text-base leading-7 text-slate-600">
              The scheduler, ingestion, notification outbox, and incident lifecycle run in a single Go service. No JVM, no Node event loop, no agent fleet.
            </p>
          </div>

          <div className="min-w-0 space-y-4">
            <div className="grid min-w-0 gap-3 sm:grid-cols-3">
              {engineeringItems.map((item) => {
                const Icon = item.icon
                return (
                  <article key={item.title} className="rounded-[8px] border border-slate-200 bg-slate-50 p-4">
                    <Icon className="h-5 w-5 text-blue-600" />
                    <h3 className="mt-3 text-base font-bold text-slate-950">{item.title}</h3>
                    <p className="mt-2 text-sm leading-6 text-slate-600">{item.copy}</p>
                  </article>
                )
              })}
            </div>

            <div className="overflow-hidden rounded-[8px] border border-slate-900 bg-slate-950">
              <div className="border-b border-white/10 px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-400">
                Production start
              </div>
              <pre className="overflow-x-auto p-4 text-sm leading-7 text-slate-200">
                <code className="whitespace-pre-wrap break-words">{productionCommand}</code>
              </pre>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
