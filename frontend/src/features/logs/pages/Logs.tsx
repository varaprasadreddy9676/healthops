import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useState } from 'react'
import { FileText, Filter, Brain, AlertTriangle, Bug, Database, Wifi, Clock, Layers } from 'lucide-react'
import { logsApi, type ErrorFamily } from "@/features/logs/api/logs"
import { LoadingState } from "@/shared/components/LoadingState"
import { ErrorState } from "@/shared/components/ErrorState"
import { EmptyState } from "@/shared/components/EmptyState"
import { MetricCard } from "@/shared/components/MetricCard"
import { cn, relativeTime } from "@/shared/lib/utils"
import { REFETCH_INTERVAL } from "@/shared/lib/constants"

const CATEGORY_ICONS: Record<string, typeof Bug> = {
    db_auth: Database,
    timeout: Clock,
    thread_exhaustion: Layers,
    slow_query: Database,
    network: Wifi,
    app_bug: Bug,
}

const SEVERITY_COLORS: Record<string, string> = {
    critical: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
    low: 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400',
}

const STATUS_COLORS: Record<string, string> = {
    active: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    resolved: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    muted: 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400',
}

function CategoryBadge({ category }: { category: string }) {
    const Icon = CATEGORY_ICONS[category] || AlertTriangle
    return (
        <span className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
            <Icon className="h-3 w-3" />
            {category.replace(/_/g, ' ')}
        </span>
    )
}

export default function Logs() {
    const queryClient = useQueryClient()
    const [statusFilter, setStatusFilter] = useState<string>('')
    const [categoryFilter, setCategoryFilter] = useState<string>('')

    const { data: families, isLoading, error, refetch } = useQuery({
        queryKey: ['logs', 'families', statusFilter, categoryFilter],
        queryFn: () => logsApi.families({ status: statusFilter || undefined, category: categoryFilter || undefined, limit: 100 }),
        refetchInterval: REFETCH_INTERVAL,
    })

    const { data: stats } = useQuery({
        queryKey: ['logs', 'stats'],
        queryFn: logsApi.stats,
        refetchInterval: REFETCH_INTERVAL,
    })

    const categorizeMutation = useMutation({
        mutationFn: () => logsApi.categorize(20),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['logs'] })
        },
    })

    if (isLoading) return <LoadingState />
    if (error) return <ErrorState message={error.message} retry={() => refetch()} />

    return (
        <div className="space-y-5 animate-fade-in">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div>
                    <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">Log Intelligence</h1>
                    <p className="text-sm text-slate-500">
                        {stats?.totalFamilies ?? 0} error families &middot; {stats?.totalEntries ?? 0} total entries
                    </p>
                </div>
                <button
                    onClick={() => categorizeMutation.mutate()}
                    disabled={categorizeMutation.isPending}
                    className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                >
                    <Brain className="h-4 w-4" />
                    {categorizeMutation.isPending ? 'Categorizing…' : 'AI Categorize'}
                </button>
            </div>

            {/* Stats cards */}
            {stats && (
                <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                    <MetricCard label="Error Families" value={stats.totalFamilies} />
                    <MetricCard label="Active" value={stats.activeFamilies} className={stats.activeFamilies > 0 ? 'ring-1 ring-red-200 dark:ring-red-900' : ''} />
                    <MetricCard label="Total Entries" value={stats.totalEntries} />
                    <MetricCard label="Categories" value={Object.keys(stats.categoryCounts).length} />
                </div>
            )}

            {/* Category breakdown */}
            {stats && Object.keys(stats.categoryCounts).length > 0 && (
                <div className="flex flex-wrap gap-2">
                    {Object.entries(stats.categoryCounts)
                        .sort(([, a], [, b]) => b - a)
                        .map(([cat, count]) => (
                            <button
                                key={cat}
                                onClick={() => setCategoryFilter(cat === categoryFilter ? '' : cat)}
                                className={cn(
                                    'inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-medium transition-colors',
                                    cat === categoryFilter
                                        ? 'bg-blue-600 text-white'
                                        : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300'
                                )}
                            >
                                {cat.replace(/_/g, ' ')} ({count})
                            </button>
                        ))}
                </div>
            )}

            {/* Filters */}
            <div className="flex items-center gap-3">
                <Filter className="h-4 w-4 text-slate-400" />
                <select
                    value={statusFilter}
                    onChange={(e) => setStatusFilter(e.target.value)}
                    className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                >
                    <option value="">All statuses</option>
                    <option value="active">Active</option>
                    <option value="resolved">Resolved</option>
                    <option value="muted">Muted</option>
                </select>
                <select
                    value={categoryFilter}
                    onChange={(e) => setCategoryFilter(e.target.value)}
                    className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                >
                    <option value="">All categories</option>
                    <option value="db_auth">DB Auth</option>
                    <option value="timeout">Timeout</option>
                    <option value="thread_exhaustion">Thread Exhaustion</option>
                    <option value="slow_query">Slow Query</option>
                    <option value="network">Network</option>
                    <option value="app_bug">App Bug</option>
                    <option value="unknown">Unknown</option>
                </select>
            </div>

            {/* Families list */}
            {families && families.length > 0 ? (
                <div className="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
                    <div className="divide-y divide-slate-100 dark:divide-slate-800">
                        {families.map((f: ErrorFamily) => (
                            <Link
                                key={f.id}
                                to={`/logs/${f.id}`}
                                className="flex items-start gap-4 px-4 py-3 transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/50"
                            >
                                <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400">
                                    <FileText className="h-4 w-4" />
                                </div>
                                <div className="min-w-0 flex-1">
                                    <div className="flex items-center gap-2">
                                        <p className="truncate text-sm font-medium text-slate-900 dark:text-slate-100">
                                            {f.pattern || f.title || f.fingerprint.slice(0, 12)}
                                        </p>
                                        {f.category && f.category !== 'unknown' && <CategoryBadge category={f.category} />}
                                        {f.severity && (
                                            <span className={cn('rounded-full px-2 py-0.5 text-xs font-medium', SEVERITY_COLORS[f.severity] || SEVERITY_COLORS.low)}>
                                                {f.severity}
                                            </span>
                                        )}
                                    </div>
                                    <div className="mt-1 flex items-center gap-3 text-xs text-slate-500">
                                        <span>{f.source}</span>
                                        <span>{f.occurrenceCount} occurrences</span>
                                        {f.servers && f.servers.length > 0 && (
                                            <span>{f.servers.length} server{f.servers.length > 1 ? 's' : ''}</span>
                                        )}
                                        <span>Last seen {relativeTime(f.lastSeenAt)}</span>
                                    </div>
                                    {f.aiSummary && (
                                        <p className="mt-1 text-xs text-slate-500 dark:text-slate-400 italic truncate">
                                            AI: {f.aiSummary}
                                        </p>
                                    )}
                                </div>
                                <div className="shrink-0">
                                    <span className={cn('rounded-full px-2 py-0.5 text-xs font-medium', STATUS_COLORS[f.status] || STATUS_COLORS.active)}>
                                        {f.status}
                                    </span>
                                </div>
                            </Link>
                        ))}
                    </div>
                </div>
            ) : (
                <EmptyState
                    icon={<FileText className="h-12 w-12 text-slate-300" />}
                    title="No error families"
                    description="Ingest logs via the API to start clustering errors automatically."
                />
            )}
        </div>
    )
}
