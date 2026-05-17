import { productScreens } from '@/features/landing/landingData'

export function ScreenshotSection() {
  return (
    <section id="screens" className="border-b border-slate-200 bg-slate-50 py-16 sm:py-20">
      <div className="mx-auto max-w-[1440px] px-4 sm:px-6 lg:px-8">
        <div className="mb-8 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 className="max-w-3xl text-3xl font-bold leading-tight text-slate-950 sm:text-5xl">
              See the workspace before you sign in.
            </h2>
            <p className="mt-4 max-w-2xl text-base leading-7 text-slate-600">
              What the team sees day to day: dashboards, logs, status pages, and incidents.
            </p>
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          {productScreens.map((screen) => (
            <article key={screen.title} className="overflow-hidden rounded-[8px] border border-slate-200 bg-white shadow-xl shadow-slate-950/5">
              <div className="border-b border-slate-200 px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h3 className="text-sm font-bold text-slate-950">{screen.title}</h3>
                    <p className="mt-0.5 text-xs font-medium text-slate-500">{screen.label}</p>
                  </div>
                </div>
              </div>
              <div className="bg-slate-100 p-2">
                <div className="overflow-hidden rounded-[6px] border border-slate-300 bg-slate-950 shadow-inner">
                  <div className="flex h-6 items-center gap-1.5 border-b border-white/10 bg-slate-900 px-2">
                    <span className="h-2 w-2 rounded-full bg-red-400" />
                    <span className="h-2 w-2 rounded-full bg-amber-300" />
                    <span className="h-2 w-2 rounded-full bg-emerald-400" />
                  </div>
                  <img
                    src={screen.image}
                    alt={`${screen.title} screenshot from HealthOps`}
                    width={1440}
                    height={1000}
                    className="aspect-[144/100] w-full bg-white object-contain"
                    loading="eager"
                    decoding="async"
                  />
                </div>
              </div>
              <p className="px-4 pb-4 pt-3 text-sm leading-6 text-slate-600">{screen.copy}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  )
}
