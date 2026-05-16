import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { useState } from 'react'
import { ArrowLeft, Server, Clock, FileText, Tag } from 'lucide-react'
import { logsApi } from "@/features/logs/api/logs"
import { LoadingState } from "@/shared/components/LoadingState"
import { ErrorState } from "@/shared/components/ErrorState"
import { cn, relativeTime } from "@/shared/lib/utils"

const SEVERITY_COLORS: Record<string, string> = {
    critical: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
    low: 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400',
}

const LEVEL_COLORS: Record<string, string> = {
    error: 'text-red-600 dark:text-red-400',
    warn: 'text-yellow-600 dark:text-yellow-400',
    warning: 'text-yellow-600 dark:text-yellow-400',
    info: 'text-blue-600 dark:text-blue-400',
    debug: 'text-slate-500',
}

export default function LogFamilyDetail() {
    const { id } = useParams<{ id: string }>()
    const queryClient = useQueryClient()
    const [editingStatus, setEditingStatus] = useState(false)

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
            setEditingStatus(false)
        },
    })

    if (isLoading) return <LoadingState />
    if (error) return <ErrorState message={error.message} retry={() => refetch()} />
    if (!data) return <ErrorState message="Family not found" />

    const { family, entries } = data

    return (
        <div className="space-y-6 animate-fade-in">
            {/* Header */}
            <div>
                <Link to="/logs" className="inline-flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 mb-3">
                    <ArrowLeft className="h-4 w-4" /> Back to Log Intelligence
                </Link>
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                        <h1 className="text-lg font-bold text-slate-900 dark:text-slate-100">
                            {family.pattern || family.title || `Family ${family.id.slice(0, 8)}`}
                        </h1>
                        <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-slate-500">
                            <span className="font-mono text-xs">{family.fingerprint.slice(0, 16)}…</span>
                            <span>&middot;</span>
                            <span>{family.occurrenceCount} occurrences</span>
                            <span>&middot;</span>
                            <span>Source: {family.source}</span>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        {family.severity && (
                            <span className={cn('rounded-full px-2.5 py-1 text-xs font-medium', SEVERITY_COLORS[family.severity] || SEVERITY_COLORS.low)}>
                                {family.severity}
                            </span>
                        )}
                        {editingStatus ? (
                            <select
                                defaultValue={family.status}
                                onChange={(e) => updateMutation.mutate({ status: e.target.value })}
                                className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
                            >
                                <option value="active">Active</option>
                                <option value="resolved">Resolved</option>
                                <option value="muted">Muted</option>
                            </select>
                        ) : (
                            <button
                                onClick={() => setEditingStatus(true)}
                                className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                            >
                                Status: {family.status}
                            </button>
                        )}
                    </div>
                </div>
            </div>

            {/* Metadata */}
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
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
                </div>
                <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
                    <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
                        <Tag className="h-3.5 w-3.5" /> Category
                    </div>
                    <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
                        {family.category?.replace(/_/g, ' ') || 'Uncategorized'}
                    </p>
                </div>
                <div className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
                    <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
                        <Server className="h-3.5 w-3.5" /> Affected Servers
                    </div>
                    <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
                        {family.servers?.join(', ') || 'Unknown'}
                    </p>
                </div>
            </div>

            {/* AI Summary */}
            {family.aiSummary && (
                <div className="rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-900 dark:bg-blue-950/30">
                    <p className="text-xs font-medium text-blue-700 dark:text-blue-400 mb-1">AI Analysis</p>
                    <p className="text-sm text-blue-800 dark:text-blue-300">{family.aiSummary}</p>
                </div>
            )}

            {/* Sample Messages */}
            {family.sampleMessages && family.sampleMessages.length > 0 && (
                <div className="rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
                    <div className="border-b border-slate-100 px-4 py-3 dark:border-slate-800">
                        <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">Sample Messages</h2>
                    </div>
                    <div className="divide-y divide-slate-100 dark:divide-slate-800">
                        {family.sampleMessages.map((msg, i) => (
                            <div key={i} className="px-4 py-2">
                                <code className="text-xs text-slate-700 dark:text-slate-300 break-all">{msg}</code>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Recent Entries */}
            <div className="rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
                <div className="border-b border-slate-100 px-4 py-3 dark:border-slate-800">
                    <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">
                        Recent Entries ({entries?.length ?? 0})
                    </h2>
                </div>
                {entries && entries.length > 0 ? (
                    <div className="divide-y divide-slate-100 dark:divide-slate-800 max-h-[400px] overflow-y-auto">
                        {entries.map((entry) => (
                            <div key={entry.id} className="flex items-start gap-3 px-4 py-2.5">
                                <FileText className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />
                                <div className="min-w-0 flex-1">
                                    <div className="flex items-center gap-2 text-xs">
                                        <span className={cn('font-medium uppercase', LEVEL_COLORS[entry.level] || 'text-slate-500')}>
                                            {entry.level}
                                        </span>
                                        <span className="text-slate-400">{new Date(entry.timestamp).toLocaleString()}</span>
                                        {entry.server && <span className="text-slate-400">[{entry.server}]</span>}
                                    </div>
                                    <p className="mt-0.5 text-xs text-slate-700 dark:text-slate-300 font-mono break-all">
                                        {entry.message}
                                    </p>
                                    {entry.stackTrace && (
                                        <details className="mt-1">
                                            <summary className="cursor-pointer text-xs text-slate-400 hover:text-slate-600">
                                                Stack trace
                                            </summary>
                                            <pre className="mt-1 overflow-x-auto rounded bg-slate-50 p-2 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-400">
                                                {entry.stackTrace}
                                            </pre>
                                        </details>
                                    )}
                                </div>
                            </div>
                        ))}
                    </div>
                ) : (
                    <div className="px-4 py-8 text-center text-sm text-slate-500">
                        No entries found for this family.
                    </div>
                )}
            </div>
        </div>
    )
}
