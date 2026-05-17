import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { useState, useMemo } from 'react'
import {
    ArrowLeft, Server, Clock, FileText, Tag, Brain, Copy, Check,
    Search, Shield, ChevronDown, TrendingUp, AlertTriangle, Activity
} from 'lucide-react'
import { logsApi } from "@/features/logs/api/logs"
import { LoadingState } from "@/shared/components/LoadingState"
import { ErrorState } from "@/shared/components/ErrorState"
import { useToast } from "@/shared/components/Toast"
import { cn, relativeTime } from "@/shared/lib/utils"
import { useAIAvailability } from "@/features/ai/hooks/useAIAvailability"

const SEVERITY_COLORS: Record<string, string> = {
    critical: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
    medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
    info: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    low: 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400',
}

const LEVEL_COLORS: Record<string, string> = {
    error: 'text-red-600 dark:text-red-400',
    warn: 'text-yellow-600 dark:text-yellow-400',
    warning: 'text-yellow-600 dark:text-yellow-400',
    info: 'text-blue-600 dark:text-blue-400',
    debug: 'text-slate-500',
}

const LEVEL_BG: Record<string, string> = {
    error: 'bg-red-50 border-red-100 dark:bg-red-900/10 dark:border-red-900/30',
    warn: 'bg-yellow-50 border-yellow-100 dark:bg-yellow-900/10 dark:border-yellow-900/30',
    warning: 'bg-yellow-50 border-yellow-100 dark:bg-yellow-900/10 dark:border-yellow-900/30',
    info: 'bg-blue-50 border-blue-100 dark:bg-blue-900/10 dark:border-blue-900/30',
    debug: 'bg-slate-50 border-slate-100 dark:bg-slate-800/50 dark:border-slate-700',
}

const CATEGORY_OPTIONS = [
    { value: 'db_auth', label: 'DB Auth' },
    { value: 'timeout', label: 'Timeout' },
    { value: 'thread_exhaustion', label: 'Thread Exhaustion' },
    { value: 'slow_query', label: 'Slow Query' },
    { value: 'database', label: 'Database' },
    { value: 'network', label: 'Network' },
    { value: 'app_bug', label: 'App Bug' },
    { value: 'application', label: 'Application' },
    { value: 'memory', label: 'Memory' },
    { value: 'config', label: 'Config' },
    { value: 'permission', label: 'Permission' },
    { value: 'security', label: 'Security' },
    { value: 'rate_limit', label: 'Rate Limit' },
    { value: 'access_log', label: 'Access Log' },
    { value: 'audit', label: 'Audit' },
    { value: 'disk_io', label: 'Disk I/O' },
    { value: 'unknown', label: 'Unclassified' },
]

function formatMetaValue(value: unknown): string {
    if (value == null) return ''
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
        return String(value)
    }
    try {
        return JSON.stringify(value)
    } catch {
        return String(value)
    }
}

function CopyButton({ text }: { text: string }) {
    const [copied, setCopied] = useState(false)
    const handleCopy = () => {
        navigator.clipboard.writeText(text)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }
    return (
        <button
            onClick={handleCopy}
            className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-700 dark:hover:text-slate-300"
            title="Copy"
        >
            {copied ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
        </button>
    )
}

function TimeWindow({ firstSeen, lastSeen }: { firstSeen: string; lastSeen: string }) {
    const first = new Date(firstSeen)
    const last = new Date(lastSeen)
    const durationMs = last.getTime() - first.getTime()
    const hours = Math.floor(durationMs / 3600000)
    const days = Math.floor(hours / 24)

    let duration = ''
    if (days > 0) duration = `${days}d ${hours % 24}h window`
    else if (hours > 0) duration = `${hours}h window`
    else duration = `${Math.max(1, Math.floor(durationMs / 60000))}m window`

    return (
        <span className="text-xs text-slate-400">
            <Activity className="mr-1 inline h-3 w-3" />
            {duration}
        </span>
    )
}

export default function LogFamilyDetail() {
    const { id } = useParams<{ id: string }>()
    const queryClient = useQueryClient()
    const toast = useToast()
    const { isAIAvailable } = useAIAvailability()
    const [editingField, setEditingField] = useState<'status' | 'category' | 'severity' | null>(null)
    const [entrySearch, setEntrySearch] = useState('')
    const [expandedEntries, setExpandedEntries] = useState<Set<string>>(new Set())

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ['logs', 'family', id],
        queryFn: () => logsApi.familyDetail(id!),
        enabled: !!id,
    })

    const updateMutation = useMutation({
        mutationFn: (patch: { status?: string; category?: string; severity?: string }) =>
            logsApi.updateFamily(id!, patch),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['logs'] })
            setEditingField(null)
            toast.success('Pattern updated')
        },
        onError: (err: Error) => toast.error(err.message || 'Update failed'),
    })

    const categorizeMutation = useMutation({
        mutationFn: () => logsApi.categorizeFamily(id!),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['logs', 'family', id] })
            queryClient.invalidateQueries({ queryKey: ['logs'] })
            toast.success('Categorized this pattern')
        },
        onError: (err: Error) => toast.error(err.message || 'Categorization failed'),
    })

    const filteredEntries = useMemo(() => {
        if (!data?.entries) return []
        if (!entrySearch.trim()) return data.entries
        const q = entrySearch.toLowerCase()
        return data.entries.filter((e) =>
            e.message.toLowerCase().includes(q) ||
            (e.server || '').toLowerCase().includes(q) ||
            e.level.toLowerCase().includes(q) ||
            (e.stackTrace || '').toLowerCase().includes(q)
        )
    }, [data?.entries, entrySearch])

    const toggleExpand = (entryId: string) => {
        setExpandedEntries((prev) => {
            const next = new Set(prev)
            if (next.has(entryId)) next.delete(entryId)
            else next.add(entryId)
            return next
        })
    }

    if (isLoading) return <LoadingState />
    if (error) return <ErrorState message={error.message} retry={() => refetch()} />
    if (!data) return <ErrorState message="Pattern not found" />

    const { family, entries } = data
    const categoryLabel = CATEGORY_OPTIONS.find((opt) => opt.value === family.category)?.label
        ?? family.category?.replace(/_/g, ' ')
        ?? 'Unclassified'

    return (
        <div className="space-y-6 animate-fade-in">
            {/* Header */}
            <div>
                <Link to="/logs" className="inline-flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 mb-3">
                    <ArrowLeft className="h-4 w-4" /> Back to Logs
                </Link>
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div className="space-y-1.5">
                        <h1 className="text-lg font-bold text-slate-900 dark:text-slate-100">
                            {family.pattern || family.title || `Pattern ${family.id.slice(0, 8)}`}
                        </h1>
                        <div className="flex flex-wrap items-center gap-2 text-sm text-slate-500">
                            <span className="inline-flex items-center gap-1 font-mono text-xs bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded">
                                {family.fingerprint.slice(0, 16)}...
                            </span>
                            <span>&middot;</span>
                            <span className="font-medium">{family.occurrenceCount.toLocaleString()} occurrences</span>
                            <span>&middot;</span>
                            <span>Source: {family.source}</span>
                            <TimeWindow firstSeen={family.firstSeenAt} lastSeen={family.lastSeenAt} />
                        </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        {isAIAvailable && (
                            <button
                                onClick={() => categorizeMutation.mutate()}
                                disabled={categorizeMutation.isPending}
                                className="inline-flex items-center gap-1.5 rounded-lg border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs font-medium text-blue-700 hover:bg-blue-100 disabled:opacity-50 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-400 dark:hover:bg-blue-900/50"
                            >
                                <Brain className="h-3.5 w-3.5" />
                                {categorizeMutation.isPending ? 'Analyzing...' : 'AI Analyze'}
                            </button>
                        )}

                        {/* Severity badge / editor */}
                        {editingField === 'severity' ? (
                            <select
                                defaultValue={family.severity}
                                onChange={(e) => updateMutation.mutate({ severity: e.target.value })}
                                onBlur={() => setEditingField(null)}
                                autoFocus
                                title="Select severity"
                                className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-800"
                            >
                                <option value="critical">Critical</option>
                                <option value="high">High</option>
                                <option value="warning">Warning</option>
                                <option value="medium">Medium</option>
                                <option value="info">Info</option>
                                <option value="low">Low</option>
                            </select>
                        ) : (
                            <button
                                onClick={() => setEditingField('severity')}
                                className={cn('rounded-full px-2.5 py-1 text-xs font-medium cursor-pointer hover:opacity-80', SEVERITY_COLORS[family.severity] || SEVERITY_COLORS.low)}
                                title="Click to change severity"
                            >
                                <Shield className="mr-1 inline h-3 w-3" />
                                {family.severity || 'unknown'}
                            </button>
                        )}

                        {/* Status badge / editor */}
                        {editingField === 'status' ? (
                            <select
                                defaultValue={family.status}
                                onChange={(e) => updateMutation.mutate({ status: e.target.value })}
                                onBlur={() => setEditingField(null)}
                                autoFocus
                                title="Select status"
                                className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-800"
                            >
                                <option value="active">Active</option>
                                <option value="resolved">Resolved</option>
                                <option value="muted">Muted</option>
                            </select>
                        ) : (
                            <button
                                onClick={() => setEditingField('status')}
                                className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                                title="Click to change status"
                            >
                                Status: <span className="font-medium capitalize">{family.status}</span>
                            </button>
                        )}
                    </div>
                </div>
            </div>

            {/* Metadata grid */}
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
                <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
                    <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
                        <Clock className="h-3.5 w-3.5" /> First Seen
                    </div>
                    <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
                        {new Date(family.firstSeenAt).toLocaleString()}
                    </p>
                </div>
                <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
                    <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
                        <Clock className="h-3.5 w-3.5" /> Last Seen
                    </div>
                    <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
                        {relativeTime(family.lastSeenAt)}
                    </p>
                    <p className="text-[10px] text-slate-400 mt-0.5">{new Date(family.lastSeenAt).toLocaleString()}</p>
                </div>
                <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
                    <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
                        <Tag className="h-3.5 w-3.5" /> Category
                    </div>
                    {editingField === 'category' ? (
                        <select
                            defaultValue={family.category || 'unknown'}
                            onChange={(e) => updateMutation.mutate({ category: e.target.value })}
                            onBlur={() => setEditingField(null)}
                            autoFocus
                            title="Select category"
                            className="w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
                        >
                            {CATEGORY_OPTIONS.map((opt) => (
                                <option key={opt.value} value={opt.value}>{opt.label}</option>
                            ))}
                        </select>
                    ) : (
                        <button
                            onClick={() => setEditingField('category')}
                            className="text-sm font-medium text-slate-900 dark:text-slate-100 hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer"
                            title="Click to change category"
                        >
                            {categoryLabel}
                        </button>
                    )}
                    {(family.category === 'unknown' || !family.category) && (
                        <p className="mt-1 text-[10px] text-slate-400">Rule-based classifier could not identify this pattern.</p>
                    )}
                </div>
                <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
                    <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
                        <Server className="h-3.5 w-3.5" /> Servers
                    </div>
                    <div className="flex flex-wrap gap-1">
                        {family.servers && family.servers.length > 0
                            ? family.servers.map((s) => (
                                <span key={s} className="rounded bg-slate-100 px-1.5 py-0.5 text-xs font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                                    {s}
                                </span>
                            ))
                            : <span className="text-sm text-slate-500">Unknown</span>
                        }
                    </div>
                </div>
                <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
                    <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
                        <TrendingUp className="h-3.5 w-3.5" /> Rate
                    </div>
                    <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
                        {(() => {
                            const first = new Date(family.firstSeenAt).getTime()
                            const last = new Date(family.lastSeenAt).getTime()
                            const hours = Math.max(1, (last - first) / 3600000)
                            const rate = family.occurrenceCount / hours
                            return rate >= 1 ? `${rate.toFixed(1)}/hr` : `${(rate * 24).toFixed(1)}/day`
                        })()}
                    </p>
                    <p className="text-[10px] text-slate-400 mt-0.5">{family.occurrenceCount.toLocaleString()} total</p>
                </div>
            </div>

            {/* AI Summary */}
            {isAIAvailable && (family.aiSummary || family.aiLabel) && (
                <div className="rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-900 dark:bg-blue-950/30">
                    <div className="flex items-center gap-2 mb-2">
                        <Brain className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                        <span className="text-xs font-semibold text-blue-700 dark:text-blue-400">AI Analysis</span>
                        {family.aiLabel && (
                            <span className="rounded-full bg-blue-100 px-2 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">
                                {family.aiLabel}
                            </span>
                        )}
                    </div>
                    <p className="text-sm text-blue-800 dark:text-blue-300">{family.aiSummary}</p>
                </div>
            )}

            {/* Sample Messages */}
            {family.sampleMessages && family.sampleMessages.length > 0 && (
                <div className="rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
                    <div className="border-b border-slate-100 px-4 py-3 dark:border-slate-800">
                        <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">
                            <AlertTriangle className="mr-1.5 inline h-4 w-4 text-amber-500" />
                            Sample Messages ({family.sampleMessages.length})
                        </h2>
                    </div>
                    <div className="divide-y divide-slate-100 dark:divide-slate-800">
                        {family.sampleMessages.map((msg, i) => (
                            <div key={i} className="group flex items-start justify-between gap-2 px-4 py-2.5">
                                <code className="flex-1 text-xs text-slate-700 dark:text-slate-300 break-all font-mono leading-relaxed">{msg}</code>
                                <CopyButton text={msg} />
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Recent Entries */}
            <div className="rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
                <div className="flex items-center justify-between border-b border-slate-100 px-4 py-3 dark:border-slate-800">
                    <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">
                        Log Entries
                        <span className="ml-2 rounded-full bg-slate-100 px-2 py-0.5 text-xs font-normal text-slate-500 dark:bg-slate-800">
                            {filteredEntries.length}{entrySearch && ` / ${entries?.length ?? 0}`}
                        </span>
                    </h2>
                    <div className="relative">
                        <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
                        <input
                            type="text"
                            value={entrySearch}
                            onChange={(e) => setEntrySearch(e.target.value)}
                            placeholder="Filter entries..."
                            className="w-48 rounded-lg border border-slate-200 bg-white py-1.5 pl-8 pr-2 text-xs text-slate-700 placeholder:text-slate-400 focus:border-blue-300 focus:outline-none focus:ring-1 focus:ring-blue-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                        />
                    </div>
                </div>
                {filteredEntries.length > 0 ? (
                    <div className="divide-y divide-slate-100 dark:divide-slate-800 max-h-[600px] overflow-y-auto">
                        {filteredEntries.map((entry) => {
                            const isExpanded = expandedEntries.has(entry.id)
                            return (
                                <div
                                    key={entry.id}
                                    className={cn('px-4 py-3 transition-colors', LEVEL_BG[entry.level] || 'bg-white dark:bg-slate-900')}
                                >
                                    <div className="flex items-start gap-3">
                                        <div className="mt-0.5 shrink-0">
                                            <span className={cn('inline-block w-14 text-center rounded px-1.5 py-0.5 text-[10px] font-bold uppercase', LEVEL_COLORS[entry.level] || 'text-slate-500')}>
                                                {entry.level}
                                            </span>
                                        </div>
                                        <div className="min-w-0 flex-1">
                                            <div className="flex items-center justify-between gap-2">
                                                <div className="flex items-center gap-2 text-xs text-slate-500">
                                                    <span className="tabular-nums">{new Date(entry.timestamp).toLocaleString()}</span>
                                                    {entry.server && (
                                                        <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-600 dark:bg-slate-700 dark:text-slate-400">
                                                            {entry.server}
                                                        </span>
                                                    )}
                                                    {entry.tags && entry.tags.length > 0 && entry.tags.map((tag) => (
                                                        <span key={tag} className="rounded bg-blue-50 px-1.5 py-0.5 text-[10px] text-blue-600 dark:bg-blue-900/30 dark:text-blue-400">
                                                            {tag}
                                                        </span>
                                                    ))}
                                                </div>
                                                <CopyButton text={entry.message} />
                                            </div>
                                            <p className="mt-1 text-xs text-slate-800 dark:text-slate-200 font-mono break-all leading-relaxed">
                                                {entry.message}
                                            </p>
                                            {entry.meta && Object.keys(entry.meta).length > 0 && (
                                                <div className="mt-1.5 flex flex-wrap gap-1.5">
                                                    {Object.entries(entry.meta).map(([k, v]) => (
                                                        <span key={k} className="inline-flex items-center rounded bg-slate-100 px-1.5 py-0.5 text-[10px] dark:bg-slate-700">
                                                            <span className="font-medium text-slate-600 dark:text-slate-400">{k}:</span>
                                                            <span className="ml-0.5 text-slate-500 dark:text-slate-500">{formatMetaValue(v)}</span>
                                                        </span>
                                                    ))}
                                                </div>
                                            )}
                                            {entry.stackTrace && (
                                                <div className="mt-2">
                                                    <button
                                                        onClick={() => toggleExpand(entry.id)}
                                                        className="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
                                                    >
                                                        <ChevronDown className={cn('h-3 w-3 transition-transform', isExpanded && 'rotate-180')} />
                                                        Stack trace
                                                    </button>
                                                    {isExpanded && (
                                                        <div className="relative mt-1.5">
                                                            <pre className="overflow-x-auto rounded-lg bg-slate-900 p-3 text-xs text-slate-300 leading-relaxed dark:bg-slate-950">
                                                                {entry.stackTrace}
                                                            </pre>
                                                            <div className="absolute right-2 top-2">
                                                                <CopyButton text={entry.stackTrace} />
                                                            </div>
                                                        </div>
                                                    )}
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                </div>
                            )
                        })}
                    </div>
                ) : (
                    <div className="px-4 py-12 text-center">
                        <FileText className="mx-auto h-8 w-8 text-slate-300" />
                        <p className="mt-2 text-sm text-slate-500">
                            {entrySearch ? 'No entries match your search.' : 'No entries found for this pattern.'}
                        </p>
                    </div>
                )}
            </div>
        </div>
    )
}
