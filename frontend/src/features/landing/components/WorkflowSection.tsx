import { workflowSteps } from '@/features/landing/landingData'

export function WorkflowSection() {
  return (
    <section id="workflow" className="border-b border-slate-800 bg-slate-950 py-16 text-white sm:py-20">
      <div className="mx-auto max-w-[1440px] px-4 sm:px-6 lg:px-8">
        <div className="grid gap-10 lg:grid-cols-[0.72fr_1fr] lg:items-start">
          <div>
            <h2 className="max-w-2xl text-3xl font-bold leading-tight sm:text-5xl">
              From failing check to recovery notification.
            </h2>
          </div>

          <div className="grid gap-3">
            {workflowSteps.map((step) => (
              <article key={step.title} className="grid gap-3 rounded-[8px] border border-white/10 bg-white/[0.05] p-4 sm:grid-cols-[72px_1fr]">
                <div className="text-2xl font-bold text-white">{step.step}</div>
                <div>
                  <h3 className="text-lg font-bold">{step.title}</h3>
                  <p className="mt-1 text-sm leading-6 text-slate-300">{step.copy}</p>
                </div>
              </article>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
