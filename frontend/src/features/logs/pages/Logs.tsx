import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useState, useMemo } from 'react'
import {
    FileText, Filter, Brain, AlertTriangle, Bug, Database, Wifi, Clock,
    Layers, Search, ChevronDown, ChevronUp, ArrowUpDown, HardDrive,
    Settings, Key, BarChart3, TrendingUp, X
} from 'lucide-react'
import { logsApi, type ErrorFamily } from "@/features/logs/api/logs"
import { LoadingState } from "@/shared/components/LoadingState"
import { ErrorState } from "@/shared/components/ErrorState"
import { EmptyState } from "@/shared/components/EmptyState"
import { MetricCard } from "@/shared/components/MetricCard"
import { useToast } from "@/shared/components/Toast"
import { cn, relativeTime } from "@/shared/lib/utils"
import { REFETCH_INTERVAL } from "@/shared/lib/constants"

const CATEGORY_ICONS: Record<string, typeof Bug> = {
    db_auth: Database,
    timeout: Clock,
    thread_exhaustion: Layers,
    slow_query: Database,
    network: Wifi,
    app_bug: Bug,
    memory: HardDrive,
    config: Settings,
    permission: Key,
    disk_io: HardDrive,
    unknown: AlertTriangle,
}

const CATEGORY_LABELS: Record<string, string> = {
    db_auth: 'DB Auth',
    timeout: 'Timeout',
    thread_exhaustion: 'Thread Exhaustion',
    slow_query: 'Slow Query',
    network: 'Network',
    app_bug: 'App Bug',
    memory: 'Memory',
    config: 'Config',
    permission: 'Permission',
    disk_io: 'Disk I/O',
    unknown: 'Unknown',
}

const SEVERITY_COLORS: Record<string, string> = {
    critical: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
    medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
    info: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    low: 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400',
}

const SEVERITY_DOT_COLORS: Record<string, string> = {
    critical: 'bg-red-500',
    high: 'bg-orange-500',
    warning: 'bg-amber-500',
    medium: 'bg-yellow-500',
    info: 'bg-blue-500',
    low: 'bg-slate-400',
}

const STATUS_COLORS: Record<string, string> = {
    active: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    resolved: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
    muted: 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400',
}

type SortField = 'occurrences' | 'lastSeen' | 'severity' | 'firstSeen'
type SortDir = 'asc' | 'desc'

const SEVERITY_ORDER: Record<string, number> = { critical: 0, high: 1, warning: 2, medium: 3, info: 4, low: 5, '': 6 }

function CategoryBadge({ category }: { category: string }) {
    const Icon = CATEGORY_ICONS[category] || AlertTriangle
    const label = CATEGORY_LABELS[category] || category.replace(/_/g, ' ')
    return (
        <span className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
            <Icon className="h-3 w-3" />
            {label}
        </span>
    )
}

function SeverityBar({ counts }: { counts: Record<string, number> }) {
    const total = Object.values(counts).reduce((a, b) => a + b, 0)
    if (total === 0) return null

    const order = ['critical', 'high', 'warning', 'medium', 'info', 'low']
    const barColors: Record<string, string> = {
        critical: 'bg-red-500',
        high: 'bg-orange-500',
        warning: 'bg-amber-500',
        medium: 'bg-yellow-400',
        info: 'bg-blue-400',
        low: 'bg-slate-300 dark:bg-slate-600',
    }

    return (
        <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
            <h3 className="mb-3 text-xs font-semibold uppercase text-slate-500">Severity Distribution</h3>
            <div className="mb-2 flex h-3 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
                {order.map((sev) => {
                    const count = counts[sev] || 0
                    if (count === 0) return null
                    const pct = (count / total) * 100
                    return (
                        <div
                            key={sev}
                            className={cn('h-full transition-all', barColors[sev])}
                            style={{ width: `${pct}%` }}
                            title={`${sev}: ${count} (${Math.round(pct)}%)`}
                        />
                    )
                })}
            </div>
            <div className="flex flex-wrap gap-3">
                {order.map((sev) => {
                    const count = counts[sev] || 0
                    if (count === 0) return null
                    return (
                        <div key={sev} className="flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-400">
                            <div className={cn('h-2 w-2 rounded-full', barColors[sev])} />
                            <span className="capitalize">{sev}</span>
                            <span className="font-medium">{count}</span>
                        </div>
                    )
                })}
            </div>
        </div>
    )
}

function CategoryChart({ counts, activeFilter, onToggle }: { counts: Record<string, number>; activeFilter: string; onToggle: (cat: string) => void }) {
    const sorted = Object.entries(counts).sort(([, a], [, b]) => b - a)
    const maxCount = sorted[0]?.[1] || 1

    return (
        <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
            <h3 className="mb-3 text-xs font-semibold uppercase text-slate-500">By Category</h3>
            <div className="space-y-2">
                {sorted.map(([cat, count]) => {
                    const Icon = CATEGORY_ICONS[cat] || AlertTriangle
                    const label = CATEGORY_LABELS[cat] || cat.replace(/_/g, ' ')
                    const pct = (count / maxCount) * 100
                    const isActive = cat === activeFilter
                    return (
                        <button
                            key={cat}
                            onClick={() => onToggle(cat)}
                            className={cn(
                                'flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors',
                                isActive ? 'bg-blue-50 ring-1 ring-blue-200 dark:bg-blue-900/20 dark:ring-blue-800' : 'hover:bg-slate-50 dark:hover:bg-slate-800/50'
                            )}
                        >
                            <Icon className="h-3.5 w-3.5 shrink-0 text-slate-500" />
                            <span className="flex-1 truncate text-xs font-medium text-slate-700 dark:text-slate-300">{label}</span>
                            <div className="flex items-center gap-2">
                                <div className="h-1.5 w-16 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-700">
                                    <div className="h-full rounded-full bg-blue-500 transition-all" style={{ width: `${pct}%` }} />
                                </div>
                                <span className="w-6 text-right text-xs font-medium text-slate-500">{count}</span>
                            </div>
                        </button>
                    )
                })}
            </div>
        </div>
    )
}

export default function Logs() {
    const queryClient = useQueryClient()
    const toast = useToast()
    const [statusFilter, setStatusFilter] = useState<string>('')
    const [categoryFilter, setCategoryFilter] = useState<string>('')
    const [severityFilter, setSeverityFilter] = useState<string>('')
    const [searchQuery, setSearchQuery] = useState('')
    const [sortField, setSortField] = useState<SortField>('lastSeen')
    const [sortDir, setSortDir] = useState<SortDir>('desc')

    const { data: families, isLoading, error, refetch } = useQuery({
        queryKey: ['logs', 'families', statusFilter, categoryFilter],
        queryFn: () => logsApi.families({ status: statusFilter || undefined, category: categoryFilter || undefined, limit: 200 }),
        refetchInterval: REFETCH_INTERVAL,
    })

    const { data: stats } = useQuery({
        queryKey: ['logs', 'stats'],
        queryFn: logsApi.stats,
        refetchInterval: REFETCH_INTERVAL,
    })

    const categorizeMutation = useMutation({
        mutationFn: () => logsApi.categorize(20),
        onSuccess: (result) => {
            queryClient.invalidateQueries({ queryKey: ['logs'] })
            toast.success(result.categorized > 0
                ? `Categorized ${result.categorized} log ${result.categorized === 1 ? 'family' : 'families'}`
                : 'No uncategorized log families found')
        },
        onError: (err: Error) => toast.error(err.message || 'AI categorization failed'),
    })

    const filteredAndSorted = useMemo(() => {
        if (!families) return []
        let result = [...families]

        // Severity filter
        if (severityFilter) {
            result = result.filter((f) => f.severity === severityFilter)
        }

        // Text search
        if (searchQuery.trim()) {
            const q = searchQuery.toLowerCase()
            result = result.filter((f) =>
                (f.pattern || '').toLowerCase().includes(q) ||
                (f.title || '').toLowerCase().includes(q) ||
                (f.source || '').toLowerCase().includes(q) ||
                (f.aiSummary || '').toLowerCase().includes(q) ||
                (f.category || '').toLowerCase().includes(q) ||
                f.servers?.some((s) => s.toLowerCase().includes(q))
            )
        }

        // Sort
        result.sort((a, b) => {
            let cmp = 0
            switch (sortField) {
                case 'occurrences':
                    cmp = a.occurrenceCount - b.occurrenceCount
                    break
                case 'lastSeen':
                    cmp = new Date(a.lastSeenAt).getTime() - new Date(b.lastSeenAt).getTime()
                    break
                case 'firstSeen':
                    cmp = new Date(a.firstSeenAt).getTime() - new Date(b.firstSeenAt).getTime()
                    break
                case 'severity':
                    cmp = (SEVERITY_ORDER[a.severity] ?? 6) - (SEVERITY_ORDER[b.severity] ?? 6)
                    break
            }
            return sortDir === 'desc' ? -cmp : cmp
        })

        return result
    }, [families, severityFilter, searchQuery, sortField, sortDir])

    const toggleSort = (field: SortField) => {
        if (sortField === field) {
            setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'))
        } else {
            setSortField(field)
            setSortDir('desc')
        }
    }

    const activeFilters = [statusFilter, categoryFilter, severityFilter, searchQuery].filter(Boolean).length
    const clearFilters = () => { setStatusFilter(''); setCategoryFilter(''); setSeverityFilter(''); setSearchQuery('') }

    if (isLoading) return <LoadingState />
    if (error) return <ErrorState message={error.message} retry={() => refetch()} />

    return (
        <div className="space-y-5 animate-fade-in">
            {/* Header */}
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div>
                    <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">Log Intelligence</h1>
                    <p className="text-sm text-slate-500">
                        {stats?.totalFamilies ?? 0} error families &middot; {stats?.activeFamilies ?? 0} active &middot; {stats?.totalEntries ?? 0} total entries
                    </p>
                </div>
                <button
                    onClick={() => categorizeMutation.mutate()}
                    disabled={categorizeMutation.isPending}
                    className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 disabled:opacity-50 dark:bg-blue-700 dark:hover:bg-blue-600"
                >
                    <Brain className="h-4 w-4" />
                    {categorizeMutation.isPending ? 'Categorizing...' : 'AI Categorize'}
                </button>
            </div>

            {/* Stats cards */}
            {stats && (
                <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                    <MetricCard
                        label="Error Families"
                        value={stats.totalFamilies}
                        icon={<BarChart3 className="h-5 w-5" />}
                    />
                    <MetricCard
                        label="Active Families"
                        value={stats.activeFamilies}
                        icon={<AlertTriangle className="h-5 w-5" />}
                        className={stats.activeFamilies > 0 ? 'ring-1 ring-red-200 dark:ring-red-900' : ''}
                    />
                    <MetricCard
                        label="Total Entries"
                        value={stats.totalEntries.toLocaleString()}
                        icon={<FileText className="h-5 w-5" />}
                    />
                    <MetricCard
                        label="Categories"
                        value={Object.keys(stats.categoryCounts).length}
                        icon={<TrendingUp className="h-5 w-5" />}
                        subValue={`${Object.keys(stats.severityCounts).length} severity levels`}
                    />
                </div>
            )}

            {/* Severity + Category breakdowns */}
            {stats && (Object.keys(stats.severityCounts).length > 0 || Object.keys(stats.categoryCounts).length > 0) && (
                <div className="grid gap-4 lg:grid-cols-2">
                    {Object.keys(stats.severityCounts).length > 0 && (
                        <SeverityBar counts={stats.severityCounts} />
                    )}
                    {Object.keys(stats.categoryCounts).length > 0 && (
                        <CategoryChart
                            counts={stats.categoryCounts}
                            activeFilter={categoryFilter}
                            onToggle={(cat) => setCategoryFilter(cat === categoryFilter ? '' : cat)}
                        />
                    )}
                </div>
            )}

            {/* Search + Filters */}
            <div className="flex flex-wrap items-center gap-3">
                <div className="relative flex-1 min-w-[200px]">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
                    <input
                        type="text"
                        placeholder="Search patterns, sources, servers..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="w-full rounded-lg border border-slate-200 bg-white py-2 pl-9 pr-3 text-sm text-slate-700 placeholder:text-slate-400 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:focus:border-blue-700 dark:focus:ring-blue-900"
                    />
                </div>
                <Filter className="h-4 w-4 text-slate-400" />
                <select
                    value={statusFilter}
                    onChange={(e) => setStatusFilter(e.target.value)}
                    title="Filter by status"
                    className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                >
                    <option value="">All statuses</option>
                    <option value="active">Active</option>
                    <option value="resolved">Resolved</option>
                    <option value="muted">Muted</option>
                </select>
                <select
                    value={severityFilter}
                    onChange={(e) => setSeverityFilter(e.target.value)}
                    title="Filter by severity"
                    className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                >
                    <option value="">All severities</option>
                    <option value="critical">Critical</option>
                    <option value="high">High</option>
                    <option value="warning">Warning</option>
                    <option value="medium">Medium</option>
                    <option value="info">Info</option>
                    <option value="low">Low</option>
                </select>
                <select
                    value={categoryFilter}
                    onChange={(e) => setCategoryFilter(e.target.value)}
                    title="Filter by category"
                    className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                >
                    <option value="">All categories</option>
                    {Object.entries(CATEGORY_LABELS).map(([key, label]) => (
                        <option key={key} value={key}>{label}</option>
                    ))}
                </select>
                {activeFilters > 0 && (
                    <button
                        onClick={clearFilters}
                        className="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-2 text-xs font-medium text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
                    >
                        <X className="h-3 w-3" />
                        Clear ({activeFilters})
                    </button>
                )}
            </div>

            {/* Sort controls + results count */}
            <div className="flex items-center justify-between">
                <p className="text-xs text-slate-500">
                    {filteredAndSorted.length} {filteredAndSorted.length === 1 ? 'family' : 'families'}
                    {activeFilters > 0 && ` (filtered from ${families?.length ?? 0})`}
                </p>
                <div className="flex items-center gap-1">
                    <ArrowUpDown className="h-3.5 w-3.5 text-slate-400" />
                    {([
                        ['lastSeen', 'Last Seen'],
                        ['occurrences', 'Count'],
                        ['severity', 'Severity'],
                        ['firstSeen', 'First Seen'],
                    ] as [SortField, string][]).map(([field, label]) => (
                        <button
                            key={field}
                            onClick={() => toggleSort(field)}
                            className={cn(
                                'inline-flex items-center gap-0.5 rounded px-2 py-1 text-xs font-medium transition-colors',
                                sortField === field
                                    ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                                    : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
                            )}
                        >
                            {label}
                            {sortField === field && (
                                sortDir === 'desc'
                                    ? <ChevronDown className="h-3 w-3" />
                                    : <ChevronUp className="h-3 w-3" />
                            )}
                        </button>
                    ))}
                </div>
            </div>

            {/* Families list */}
            {filteredAndSorted.length > 0 ? (
                <div className="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
                    <div className="divide-y divide-slate-100 dark:divide-slate-800">
                        {filteredAndSorted.map((f: ErrorFamily) => {
                            const SevDot = SEVERITY_DOT_COLORS[f.severity] || SEVERITY_DOT_COLORS.low
                            return (
                                <Link
                                    key={f.id}
                                    to={`/logs/${f.id}`}
                                    className="group flex items-start gap-4 px-4 py-3.5 transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/50"
                                >
                                    {/* Severity dot + icon */}
                                    <div className="relative mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400">
                                        <FileText className="h-4 w-4" />
                                        <div className={cn('absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full ring-2 ring-white dark:ring-slate-900', SevDot)} />
                                    </div>

                                    {/* Content */}
                                    <div className="min-w-0 flex-1">
                                        <div className="flex items-center gap-2">
                                            <p className="truncate text-sm font-medium text-slate-900 group-hover:text-blue-700 dark:text-slate-100 dark:group-hover:text-blue-400">
                                                {f.pattern || f.title || f.fingerprint.slice(0, 16)}
                                            </p>
                                            {f.category && f.category !== 'unknown' && <CategoryBadge category={f.category} />}
                                            {f.severity && (
                                                <span className={cn('rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase', SEVERITY_COLORS[f.severity] || SEVERITY_COLORS.low)}>
                                                    {f.severity}
                                                </span>
                                            )}
                                        </div>
                                        <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-slate-500">
                                            <span className="font-medium">{f.source}</span>
                                            <span className="tabular-nums">{f.occurrenceCount.toLocaleString()} occurrences</span>
                                            {f.servers && f.servers.length > 0 && (
                                                <span>{f.servers.length} server{f.servers.length > 1 ? 's' : ''}</span>
                                            )}
                                            <span>Last seen {relativeTime(f.lastSeenAt)}</span>
                                        </div>
                                        {f.aiSummary && (
                                            <p className="mt-1 truncate text-xs italic text-slate-400 dark:text-slate-500">
                                                {f.aiSummary}
                                            </p>
                                        )}
                                    </div>

                                    {/* Status badge */}
                                    <div className="shrink-0 pt-0.5">
                                        <span className={cn('rounded-full px-2.5 py-0.5 text-[10px] font-semibold uppercase', STATUS_COLORS[f.status] || STATUS_COLORS.active)}>
                                            {f.status}
                                        </span>
                                    </div>
                                </Link>
                            )
                        })}
                    </div>
                </div>
            ) : (
                <EmptyState
                    icon={<FileText className="h-12 w-12 text-slate-300" />}
                    title={activeFilters > 0 ? 'No matching families' : 'No error families'}
                    description={activeFilters > 0
                        ? 'Try adjusting your filters or search query.'
                        : 'Ingest logs via POST /api/v1/logs/ingest to start clustering errors automatically.'}
                />
            )}
        </div>
    )
}
