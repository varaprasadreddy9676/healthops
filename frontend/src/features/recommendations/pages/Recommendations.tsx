import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    AlertTriangle,
    TrendingUp,
    Shield,
    Eye,
    X,
    RefreshCw,
    Sparkles,
    Server,
    Activity,
    ChevronDown,
    ChevronUp,
} from 'lucide-react'
import { recommendationsApi, type Recommendation } from '../api/recommendations'
import { useAIAvailability } from '@/features/ai/hooks/useAIAvailability'

const CATEGORY_META: Record<string, { label: string; icon: typeof AlertTriangle; color: string }> = {
    threshold: { label: 'Threshold Tuning', icon: TrendingUp, color: 'text-blue-600 bg-blue-50 dark:text-blue-400 dark:bg-blue-950/40' },
    coverage: { label: 'Coverage Gap', icon: Shield, color: 'text-amber-600 bg-amber-50 dark:text-amber-400 dark:bg-amber-950/40' },
    stuck: { label: 'Stuck Detection', icon: AlertTriangle, color: 'text-red-600 bg-red-50 dark:text-red-400 dark:bg-red-950/40' },
}

const PRIORITY_STYLES: Record<string, string> = {
    high: 'border-l-red-500',
    medium: 'border-l-amber-500',
    low: 'border-l-blue-500',
}

function RecommendationCard({ rec, onDismiss }: { rec: Recommendation; onDismiss: (id: string) => void }) {
    const [expanded, setExpanded] = useState(false)
    const meta = CATEGORY_META[rec.category] ?? CATEGORY_META.threshold
    const Icon = meta.icon

    return (
        <div className={`rounded-lg border border-slate-200 border-l-4 ${PRIORITY_STYLES[rec.priority]} bg-white p-4 transition-shadow hover:shadow-md dark:border-slate-700 dark:bg-slate-800`}>
            <div className="flex items-start justify-between gap-3">
                <div className="flex items-start gap-3">
                    <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${meta.color}`}>
                        <Icon className="h-4 w-4" />
                    </div>
                    <div className="min-w-0">
                        <div className="flex items-center gap-2">
                            <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-200">{rec.title}</h3>
                            <span className={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium uppercase ${meta.color}`}>
                                {meta.label}
                            </span>
                        </div>
                        <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">{rec.reason}</p>
                        {rec.server && (
                            <div className="mt-1.5 flex items-center gap-1 text-xs text-slate-500">
                                <Server className="h-3 w-3" />
                                {rec.server}
                            </div>
                        )}
                    </div>
                </div>
                <div className="flex items-center gap-1">
                    <button
                        onClick={() => setExpanded(!expanded)}
                        className="rounded p-1 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-700"
                        title="Details"
                    >
                        {expanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                    </button>
                    <button
                        onClick={() => onDismiss(rec.id)}
                        className="rounded p-1 text-slate-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-950/30"
                        title="Dismiss"
                    >
                        <X className="h-4 w-4" />
                    </button>
                </div>
            </div>

            {expanded && (
                <div className="mt-3 space-y-2 border-t border-slate-100 pt-3 dark:border-slate-700">
                    <p className="whitespace-pre-wrap text-xs text-slate-600 dark:text-slate-400">{rec.description}</p>
                    {rec.current && (
                        <div className="rounded bg-slate-50 p-2 dark:bg-slate-900">
                            <p className="text-[10px] font-medium uppercase text-slate-500">Current</p>
                            <pre className="mt-1 text-xs text-slate-700 dark:text-slate-300">{JSON.stringify(rec.current, null, 2)}</pre>
                        </div>
                    )}
                    {rec.suggested && (
                        <div className="rounded bg-green-50 p-2 dark:bg-green-950/20">
                            <p className="text-[10px] font-medium uppercase text-green-600 dark:text-green-400">Suggested</p>
                            <pre className="mt-1 text-xs text-green-700 dark:text-green-300">{JSON.stringify(rec.suggested, null, 2)}</pre>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}

export default function Recommendations() {
    const [filter, setFilter] = useState<string>('')
    const queryClient = useQueryClient()
    const { isAIAvailable } = useAIAvailability()

    const { data, isLoading } = useQuery({
        queryKey: ['recommendations', filter],
        queryFn: () => recommendationsApi.list(filter || undefined),
    })

    const generateMutation = useMutation({
        mutationFn: (useAi: boolean) => recommendationsApi.generate(useAi),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['recommendations'] }),
    })

    const dismissMutation = useMutation({
        mutationFn: (id: string) => recommendationsApi.dismiss(id),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['recommendations'] }),
    })

    const recs = data?.recommendations ?? []
    const highCount = recs.filter((r) => r.priority === 'high').length
    const medCount = recs.filter((r) => r.priority === 'medium').length
    const lowCount = recs.filter((r) => r.priority === 'low').length
    const subtitle = isAIAvailable
        ? 'Threshold, coverage, and stuck-check suggestions. Add AI context when incidents need a second pass.'
        : 'Threshold, coverage, and stuck-check suggestions.'
    const emptyDescription = isAIAvailable
        ? 'Refresh rules after adding checks, or add AI context when incidents need a second pass.'
        : 'Refresh rules after adding checks.'

    return (
        <div className="flex h-full flex-col">
            {/* Header */}
            <div className="border-b border-slate-200 px-6 py-4 dark:border-slate-700">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-emerald-500 to-teal-600 text-white">
                            <Eye className="h-5 w-5" />
                        </div>
                        <div>
                            <h1 className="text-lg font-bold text-slate-800 dark:text-white">Monitor Tuning</h1>
                            <p className="text-xs text-slate-500">{subtitle}</p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <button
                            onClick={() => generateMutation.mutate(false)}
                            disabled={generateMutation.isPending}
                            className="flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-600 transition-colors hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800"
                        >
                            <RefreshCw className={`h-3.5 w-3.5 ${generateMutation.isPending ? 'animate-spin' : ''}`} />
                            Refresh Rules
                        </button>
                        {isAIAvailable && (
                            <button
                                onClick={() => generateMutation.mutate(true)}
                                disabled={generateMutation.isPending}
                                className="flex items-center gap-1.5 rounded-lg bg-violet-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-violet-700 disabled:opacity-50"
                            >
                                <Sparkles className="h-3.5 w-3.5" />
                                Add AI Context
                            </button>
                        )}
                    </div>
                </div>

                {/* Stats bar */}
                <div className="mt-3 flex items-center gap-4">
                    <div className="flex items-center gap-1.5 text-xs">
                        <Activity className="h-3.5 w-3.5 text-slate-400" />
                        <span className="text-slate-600 dark:text-slate-400">{recs.length} active</span>
                    </div>
                    {highCount > 0 && (
                        <span className="inline-flex items-center gap-1 rounded-full bg-red-50 px-2 py-0.5 text-[11px] font-medium text-red-700 dark:bg-red-950/30 dark:text-red-400">
                            {highCount} high
                        </span>
                    )}
                    {medCount > 0 && (
                        <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:bg-amber-950/30 dark:text-amber-400">
                            {medCount} medium
                        </span>
                    )}
                    {lowCount > 0 && (
                        <span className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-950/30 dark:text-blue-400">
                            {lowCount} low
                        </span>
                    )}
                </div>

                {/* Filter tabs */}
                <div className="mt-3 flex gap-1">
                    {[
                        { value: '', label: 'All' },
                        { value: 'threshold', label: 'Thresholds' },
                        { value: 'coverage', label: 'Coverage' },
                        { value: 'stuck', label: 'Stuck' },
                    ].map((tab) => (
                        <button
                            key={tab.value}
                            onClick={() => setFilter(tab.value)}
                            className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${filter === tab.value
                                ? 'bg-slate-800 text-white dark:bg-slate-200 dark:text-slate-900'
                                : 'text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800'
                                }`}
                        >
                            {tab.label}
                        </button>
                    ))}
                </div>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-5">
                {isLoading ? (
                    <div className="flex flex-col items-center justify-center py-16">
                        <RefreshCw className="h-6 w-6 animate-spin text-slate-400" />
                        <p className="mt-2 text-sm text-slate-500">Analyzing your infrastructure...</p>
                    </div>
                ) : recs.length === 0 ? (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-green-50 dark:bg-green-950/30">
                            <Shield className="h-6 w-6 text-green-500" />
                        </div>
                        <p className="mt-3 text-sm font-medium text-slate-700 dark:text-slate-300">No tuning recommendations right now</p>
                        <p className="mt-1 text-xs text-slate-500">{emptyDescription}</p>
                    </div>
                ) : (
                    <div className="space-y-3">
                        {recs.map((rec) => (
                            <RecommendationCard
                                key={rec.id}
                                rec={rec}
                                onDismiss={(id) => dismissMutation.mutate(id)}
                            />
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
