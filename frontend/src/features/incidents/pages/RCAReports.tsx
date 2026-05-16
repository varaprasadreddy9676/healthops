import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useState } from 'react'
import { Activity, Filter, CheckCircle, XCircle, Clock, TrendingUp } from 'lucide-react'
import { rcaApi, type RCAReport } from '@/features/incidents/api/rca'
import { LoadingState } from '@/shared/components/LoadingState'
import { ErrorState } from '@/shared/components/ErrorState'
import { EmptyState } from '@/shared/components/EmptyState'
import { cn, formatDate, relativeTime } from '@/shared/lib/utils'
import { MetricCard } from '@/shared/components/MetricCard'

const STATUS_CONFIG: Record<string, { icon: typeof CheckCircle; color: string; label: string }> = {
    complete: { icon: CheckCircle, color: 'text-emerald-600', label: 'Complete' },
    failed: { icon: XCircle, color: 'text-red-500', label: 'Failed' },
    pending: { icon: Clock, color: 'text-amber-500', label: 'Pending' },
    processing: { icon: Activity, color: 'text-blue-500', label: 'Processing' },
}

function ReportRow({ report }: { report: RCAReport }) {
    const statusCfg = STATUS_CONFIG[report.status] || STATUS_CONFIG.pending
    const StatusIcon = statusCfg.icon
    const topHypothesis = report.hypotheses?.[0]

    return (
        <Link
            to={`/incidents/${report.incidentId}`}
            className="group flex items-start gap-4 rounded-lg border border-slate-200 bg-white p-4 transition-all hover:border-violet-300 hover:shadow-sm dark:border-slate-700 dark:bg-slate-800/60 dark:hover:border-violet-700"
        >
            <div className={cn('mt-0.5 shrink-0', statusCfg.color)}>
                <StatusIcon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                    <p className="text-sm font-medium text-slate-800 dark:text-slate-200 group-hover:text-violet-700 dark:group-hover:text-violet-400">
                        Incident {report.incidentId.slice(0, 8)}…
                    </p>
                    <span className={cn('text-[10px] font-semibold uppercase', statusCfg.color)}>
                        {statusCfg.label}
                    </span>
                </div>
                {report.summary && (
                    <p className="mt-1 line-clamp-2 text-xs text-slate-600 dark:text-slate-400">
                        {report.summary}
                    </p>
                )}
                {topHypothesis && (
                    <div className="mt-2 flex items-center gap-2">
                        <span className="inline-flex items-center gap-1 rounded bg-violet-100 px-1.5 py-0.5 text-[10px] font-medium text-violet-700 dark:bg-violet-900/40 dark:text-violet-400">
                            #{topHypothesis.rank} {topHypothesis.title}
                        </span>
                        <span className="text-[10px] text-slate-400">
                            {Math.round(topHypothesis.confidence * 100)}% confidence
                        </span>
                    </div>
                )}
                <div className="mt-2 flex flex-wrap items-center gap-3 text-[10px] text-slate-400">
                    <span>{report.signalCount} signals</span>
                    <span>{report.hypotheses?.length ?? 0} hypotheses</span>
                    {report.providerUsed && <span>via {report.providerUsed}</span>}
                    <span title={formatDate(report.createdAt)}>{relativeTime(report.createdAt)}</span>
                </div>
            </div>
        </Link>
    )
}

export default function RCAReports() {
    const [statusFilter, setStatusFilter] = useState<string>('')
    const { data: reports, isLoading, error, refetch } = useQuery({
        queryKey: ['rca', 'all-reports'],
        queryFn: () => rcaApi.allReports(100),
    })

    const filtered = reports?.filter((r) => !statusFilter || r.status === statusFilter) ?? []
    const completedCount = reports?.filter((r) => r.status === 'complete').length ?? 0
    const failedCount = reports?.filter((r) => r.status === 'failed').length ?? 0
    const avgSignals = reports?.length
        ? Math.round(reports.reduce((s, r) => s + r.signalCount, 0) / reports.length)
        : 0
    const avgHypotheses = reports?.length
        ? (reports.reduce((s, r) => s + (r.hypotheses?.length ?? 0), 0) / reports.length).toFixed(1)
        : '0'

    if (isLoading) return <LoadingState />
    if (error) return <ErrorState message={error.message} retry={() => refetch()} />

    return (
        <div className="space-y-5 animate-fade-in">
            {/* Header */}
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div>
                    <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">Root Cause Analysis</h1>
                    <p className="text-sm text-slate-500">{reports?.length ?? 0} analyses across all incidents</p>
                </div>
            </div>

            {/* Metrics */}
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <MetricCard
                    label="Completed"
                    value={completedCount}
                    icon={<CheckCircle className="h-4 w-4 text-emerald-500" />}
                />
                <MetricCard
                    label="Failed"
                    value={failedCount}
                    icon={<XCircle className="h-4 w-4 text-red-500" />}
                />
                <MetricCard
                    label="Avg Signals"
                    value={avgSignals}
                    icon={<TrendingUp className="h-4 w-4 text-blue-500" />}
                />
                <MetricCard
                    label="Avg Hypotheses"
                    value={avgHypotheses}
                    icon={<Activity className="h-4 w-4 text-violet-500" />}
                />
            </div>

            {/* Filters */}
            <div className="flex items-center gap-2">
                <Filter className="h-3.5 w-3.5 text-slate-400" />
                {['', 'complete', 'failed', 'pending'].map((status) => (
                    <button
                        key={status}
                        onClick={() => setStatusFilter(status)}
                        className={cn(
                            'rounded-md px-2.5 py-1 text-xs font-medium transition-colors',
                            statusFilter === status
                                ? 'bg-violet-100 text-violet-700 dark:bg-violet-900/50 dark:text-violet-400'
                                : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
                        )}
                    >
                        {status === '' ? 'All' : status.charAt(0).toUpperCase() + status.slice(1)}
                    </button>
                ))}
            </div>

            {/* Reports List */}
            {filtered.length === 0 ? (
                <EmptyState
                    title="No RCA reports"
                    message="Root cause analysis reports will appear here after running RCA on an incident."
                />
            ) : (
                <div className="space-y-3">
                    {filtered.map((report) => (
                        <ReportRow key={report.id} report={report} />
                    ))}
                </div>
            )}
        </div>
    )
}
