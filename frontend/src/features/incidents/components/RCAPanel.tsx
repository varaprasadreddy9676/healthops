import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Activity, ChevronDown, Zap, AlertTriangle, Database, Network, Settings, Code } from 'lucide-react'
import { cn, formatDate } from '@/shared/lib/utils'
import { rcaApi, type RCAReport, type RCAHypothesis, type TimelineEvent } from '@/features/incidents/api/rca'
import { SignalGrid } from '@/features/incidents/components/SignalChart'

const categoryIcons: Record<string, typeof Activity> = {
    resource: Zap,
    network: Network,
    application: Code,
    database: Database,
    config: Settings,
}

function ConfidenceBar({ value }: { value: number }) {
    const pct = Math.round(value * 100)
    const color = pct >= 80 ? 'bg-emerald-500' : pct >= 50 ? 'bg-amber-500' : 'bg-red-500'
    return (
        <div className="flex items-center gap-2">
            <div className="h-1.5 w-24 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700">
                <div className={cn('h-full rounded-full transition-all', color)} style={{ width: `${pct}%` }} />
            </div>
            <span className="text-xs font-medium text-slate-500">{pct}%</span>
        </div>
    )
}

function HypothesisCard({ hypothesis }: { hypothesis: RCAHypothesis }) {
    const [expanded, setExpanded] = useState(false)
    const Icon = categoryIcons[hypothesis.category] || AlertTriangle

    return (
        <div className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-800/60">
            <div className="flex items-start justify-between gap-3">
                <div className="flex items-start gap-3">
                    <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-slate-100 dark:bg-slate-700">
                        <Icon className="h-3.5 w-3.5 text-slate-600 dark:text-slate-400" />
                    </div>
                    <div className="min-w-0">
                        <div className="flex items-center gap-2">
                            <span className="text-xs font-bold text-slate-400">#{hypothesis.rank}</span>
                            <h4 className="text-sm font-semibold text-slate-900 dark:text-slate-100">{hypothesis.title}</h4>
                        </div>
                        <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">{hypothesis.description}</p>
                    </div>
                </div>
                <ConfidenceBar value={hypothesis.confidence} />
            </div>

            {(hypothesis.evidence?.length > 0 || hypothesis.suggestion) && (
                <div className="mt-3">
                    <button
                        onClick={() => setExpanded(!expanded)}
                        className="flex items-center gap-1 text-xs font-medium text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
                    >
                        <ChevronDown className={cn('h-3 w-3 transition-transform', expanded && 'rotate-180')} />
                        {expanded ? 'Less' : 'Evidence & suggestion'}
                    </button>
                    {expanded && (
                        <div className="mt-2 space-y-2 rounded-md bg-slate-50 p-3 dark:bg-slate-900/50">
                            {hypothesis.evidence?.length > 0 && (
                                <div>
                                    <span className="text-[10px] font-semibold uppercase text-slate-400">Evidence</span>
                                    <ul className="mt-1 space-y-0.5">
                                        {hypothesis.evidence.map((e, i) => (
                                            <li key={i} className="text-xs text-slate-600 dark:text-slate-400">- {e}</li>
                                        ))}
                                    </ul>
                                </div>
                            )}
                            {hypothesis.suggestion && (
                                <div>
                                    <span className="text-[10px] font-semibold uppercase text-slate-400">Suggestion</span>
                                    <p className="mt-0.5 text-xs font-medium text-slate-700 dark:text-slate-300">{hypothesis.suggestion}</p>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}

function EventTimeline({ events }: { events: TimelineEvent[] }) {
    if (!events?.length) return null

    const typeColors: Record<string, string> = {
        incident_open: 'bg-red-500',
        check_fail: 'bg-orange-500',
        check_recover: 'bg-emerald-500',
        log_spike: 'bg-purple-500',
        metric_anomaly: 'bg-blue-500',
    }

    return (
        <div className="space-y-2">
            {events.slice(0, 15).map((event, i) => (
                <div key={i} className="flex items-start gap-2.5">
                    <div className="relative flex flex-col items-center">
                        <div className={cn('h-2 w-2 rounded-full', typeColors[event.type] || 'bg-slate-400')} />
                        {i < Math.min(events.length, 15) - 1 && (
                            <div className="absolute top-2.5 h-full w-px bg-slate-200 dark:bg-slate-700" />
                        )}
                    </div>
                    <div className="min-w-0 pb-3">
                        <div className="flex items-center gap-2">
                            <span className="text-[10px] font-medium text-slate-400">{formatDate(event.timestamp)}</span>
                            <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500 dark:bg-slate-800">
                                {event.type.replace(/_/g, ' ')}
                            </span>
                        </div>
                        <p className="mt-0.5 text-xs text-slate-600 dark:text-slate-400">{event.description}</p>
                    </div>
                </div>
            ))}
        </div>
    )
}

export function RCAPanel({ incidentId, aiEnabled }: { incidentId: string; aiEnabled: boolean }) {
    const queryClient = useQueryClient()

    const { data: reports, isLoading } = useQuery({
        queryKey: ['rca', 'reports', incidentId],
        queryFn: () => rcaApi.reports(incidentId),
        enabled: !!incidentId,
        retry: false,
    })

    const { data: timelineData } = useQuery({
        queryKey: ['rca', 'timeline', incidentId],
        queryFn: () => rcaApi.timeline(incidentId),
        enabled: !!incidentId,
        retry: false,
    })

    const analyzeMutation = useMutation({
        mutationFn: () => rcaApi.analyze(incidentId),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['rca', 'reports', incidentId] })
            queryClient.invalidateQueries({ queryKey: ['rca', 'timeline', incidentId] })
        },
    })

    const latestReport = reports?.[reports.length - 1] as RCAReport | undefined

    if (!aiEnabled && !reports?.length) return null

    return (
        <div className="rounded-xl border border-violet-200 bg-gradient-to-br from-violet-50/80 to-purple-50/40 p-5 dark:border-violet-900 dark:from-violet-950/30 dark:to-purple-950/20">
            {/* Header */}
            <div className="mb-4 flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <Activity className="h-4 w-4 text-violet-600 dark:text-violet-400" />
                    <h2 className="text-sm font-semibold text-violet-900 dark:text-violet-300">Root Cause Analysis</h2>
                </div>
                {aiEnabled && (
                    <button
                        onClick={() => analyzeMutation.mutate()}
                        disabled={analyzeMutation.isPending}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-violet-200 bg-white/80 px-3 py-1.5 text-xs font-medium text-violet-700 transition-colors hover:bg-violet-50 disabled:opacity-50 dark:border-violet-800 dark:bg-violet-950/50 dark:text-violet-400"
                    >
                        {analyzeMutation.isPending ? 'Analyzing...' : 'Run RCA'}
                    </button>
                )}
            </div>

            {isLoading && <p className="text-xs text-slate-500">Loading...</p>}

            {!isLoading && !latestReport && (
                <p className="text-xs text-slate-500 dark:text-slate-400">
                    No root cause analysis yet. Click "Run RCA" to correlate signals and identify probable causes.
                </p>
            )}

            {latestReport?.status === 'failed' && (
                <div className="rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-900 dark:bg-red-950/30">
                    <p className="text-xs font-medium text-red-700 dark:text-red-400">Analysis failed: {latestReport.error}</p>
                </div>
            )}

            {latestReport?.status === 'complete' && (
                <div className="space-y-4">
                    {/* Summary */}
                    {latestReport.summary && (
                        <p className="text-sm text-slate-700 dark:text-slate-300">{latestReport.summary}</p>
                    )}

                    {/* Signal count + window */}
                    <div className="flex flex-wrap gap-3 text-[10px] font-medium uppercase text-slate-400">
                        <span>{latestReport.signalCount} signals correlated</span>
                        <span>{latestReport.hypotheses?.length ?? 0} hypotheses</span>
                        <span>{formatDate(latestReport.createdAt)}</span>
                    </div>

                    {/* Hypotheses */}
                    {latestReport.hypotheses && latestReport.hypotheses.length > 0 && (
                        <div className="space-y-3">
                            {latestReport.hypotheses
                                .sort((a, b) => a.rank - b.rank)
                                .map((h) => (
                                    <HypothesisCard key={h.rank} hypothesis={h} />
                                ))}
                        </div>
                    )}

                    {/* Correlated Signals Grid */}
                    {timelineData?.signals && timelineData.signals.length > 0 && (
                        <SignalGrid signals={timelineData.signals} />
                    )}

                    {/* Correlated Events Timeline */}
                    {latestReport.timeline && latestReport.timeline.length > 0 && (
                        <details className="group">
                            <summary className="cursor-pointer text-xs font-medium text-violet-600 hover:text-violet-800 dark:text-violet-400 dark:hover:text-violet-300">
                                Correlated event timeline ({latestReport.timeline.length} events)
                            </summary>
                            <div className="mt-3">
                                <EventTimeline events={latestReport.timeline} />
                            </div>
                        </details>
                    )}
                </div>
            )}
        </div>
    )
}
