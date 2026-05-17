import { featureItems } from '@/features/landing/landingData'

export function CapabilitiesSection() {
  return (
    <section id="capabilities" className="border-b border-slate-200 bg-white py-16 sm:py-20">
      <div className="mx-auto max-w-[1440px] px-4 sm:px-6 lg:px-8">
        <div className="grid gap-8 lg:grid-cols-[0.62fr_1fr]">
          <div>
            <h2 className="max-w-xl text-3xl font-bold leading-tight text-slate-950 sm:text-5xl">
              One operating surface for the first 15 minutes of an incident.
            </h2>
            <p className="mt-4 max-w-xl text-base leading-7 text-slate-600">
              A check fails, an incident opens, auto-heal fixes it, the right people get notified, and recovery closes the loop. The goal is fewer tabs and less toil when something breaks.
            </p>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            {featureItems.map((item) => {
              const Icon = item.icon
              return (
                <article key={item.title} className="rounded-[8px] border border-slate-200 bg-slate-50 p-4">
                  <div className="flex h-10 w-10 items-center justify-center rounded-[8px] bg-slate-950 text-white">
                    <Icon className="h-[18px] w-[18px]" />
                  </div>
                  <h3 className="mt-4 text-base font-bold text-slate-950">{item.title}</h3>
                  <p className="mt-2 text-sm leading-6 text-slate-600">{item.copy}</p>
                </article>
              )
            })}
          </div>
        </div>
      </div>
    </section>
  )
}
