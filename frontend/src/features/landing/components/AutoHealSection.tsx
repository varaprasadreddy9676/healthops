import { Wrench, RefreshCw, ShieldCheck, Zap } from 'lucide-react'

const steps = [
    {
        icon: Zap,
        title: 'Trigger',
        copy: 'A check fails and an incident opens. The remediation engine picks up the configured action automatically.',
    },
    {
        icon: Wrench,
        title: 'Execute',
        copy: 'Runs an SSH command, shell script, or HTTP webhook on the target server. Output and exit code are captured.',
    },
    {
        icon: RefreshCw,
        title: 'Verify',
        copy: 'Waits the configured delay, then re-runs the original check. If healthy, the incident auto-resolves.',
    },
    {
        icon: ShieldCheck,
        title: 'Escalate',
        copy: 'If max attempts exhaust without recovery, the engine stops and escalates. AI explains why the fix did not work.',
    },
]

export function AutoHealSection() {
    return (
        <section id="auto-heal" className="border-b border-slate-200 bg-gradient-to-b from-amber-50 to-white py-16 sm:py-20">
            <div className="mx-auto max-w-[1440px] px-4 sm:px-6 lg:px-8">
                <div className="grid gap-10 lg:grid-cols-[0.62fr_1fr] lg:items-start">
                    <div>
                        <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-amber-200 bg-amber-100 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-amber-800">
                            <Wrench className="h-3.5 w-3.5" />
                            Auto-Heal
                        </div>
                        <h2 className="max-w-xl text-3xl font-bold leading-tight text-slate-950 sm:text-5xl">
                            Fix it before the on-call wakes up.
                        </h2>
                        <p className="mt-4 max-w-xl text-base leading-7 text-slate-600">
                            Attach a remediation action to any check. When the check fails, HealthOps
                            runs the fix, verifies recovery, and resolves the incident — all within seconds.
                            No Rundeck, no StackStorm, no enterprise add-on.
                        </p>
                        <div className="mt-6 space-y-2 text-sm text-slate-600">
                            <div className="flex items-start gap-2">
                                <span className="mt-0.5 inline-block h-1.5 w-1.5 flex-shrink-0 rounded-full bg-amber-500" />
                                <span><strong>3 action types:</strong> SSH command, shell command, HTTP webhook</span>
                            </div>
                            <div className="flex items-start gap-2">
                                <span className="mt-0.5 inline-block h-1.5 w-1.5 flex-shrink-0 rounded-full bg-amber-500" />
                                <span><strong>Safety rails:</strong> max attempts, cooldown, dry-run mode, risk tagging</span>
                            </div>
                            <div className="flex items-start gap-2">
                                <span className="mt-0.5 inline-block h-1.5 w-1.5 flex-shrink-0 rounded-full bg-amber-500" />
                                <span><strong>AI fallback:</strong> when the fix fails, AI analyzes why and suggests next steps</span>
                            </div>
                            <div className="flex items-start gap-2">
                                <span className="mt-0.5 inline-block h-1.5 w-1.5 flex-shrink-0 rounded-full bg-amber-500" />
                                <span><strong>Full audit trail:</strong> every attempt logged with command, output, exit code, and duration</span>
                            </div>
                        </div>
                    </div>

                    <div className="grid gap-3 sm:grid-cols-2">
                        {steps.map((step) => {
                            const Icon = step.icon
                            return (
                                <article key={step.title} className="rounded-[8px] border border-amber-200/60 bg-white p-4 shadow-sm">
                                    <div className="flex h-10 w-10 items-center justify-center rounded-[8px] bg-amber-500 text-white">
                                        <Icon className="h-[18px] w-[18px]" />
                                    </div>
                                    <h3 className="mt-4 text-base font-bold text-slate-950">{step.title}</h3>
                                    <p className="mt-2 text-sm leading-6 text-slate-600">{step.copy}</p>
                                </article>
                            )
                        })}
                    </div>
                </div>
            </div>
        </section>
    )
}
