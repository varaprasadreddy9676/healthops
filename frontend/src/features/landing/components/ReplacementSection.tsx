import { replacementItems } from '@/features/landing/landingData'

export function ReplacementSection() {
  return (
    <section id="replacement" className="border-b border-slate-200 bg-slate-50 py-16 sm:py-20">
      <div className="mx-auto max-w-[1440px] px-4 sm:px-6 lg:px-8">
        <div className="mb-8 max-w-3xl">
          <h2 className="text-3xl font-bold leading-tight text-slate-950 sm:text-5xl">
            A smaller stack for teams that do not need a full APM suite.
          </h2>
          <p className="mt-4 text-base leading-7 text-slate-600">
            HealthOps is not trying to be Datadog or New Relic. It replaces the duct tape: UptimeRobot-style checks, a status page, a response channel, and a MySQL CLI tab.
          </p>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {replacementItems.map((item) => (
            <article key={item.category} className="rounded-[8px] border border-slate-200 bg-white p-4 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{item.category}</p>
              <h3 className="mt-2 text-base font-bold leading-6 text-slate-950">{item.examples}</h3>
              <p className="mt-3 text-sm leading-6 text-slate-600">{item.answer}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  )
}
