import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Wrench,
    Play,
    CheckCircle2,
    XCircle,
    Clock,
    AlertTriangle,
    Eye,
    Sparkles,
    ChevronDown,
    ChevronUp,
    ToggleLeft,
    ToggleRight,
} from 'lucide-react'
import { remediationApi, type RemediationAttempt, type RemediationConfig } from '../api/remediation'
import { useToast } from '@/shared/components/Toast'
import { cn } from '@/shared/lib/utils'

const STATUS_CONFIG: Record<string, { icon: typeof CheckCircle2; color: string; label: string }> = {
    success: { icon: CheckCircle2, color: 'text-green-600 dark:text-green-400', label: 'Success' },
    failed: { icon: XCircle, color: 'text-red-600 dark:text-red-400', label: 'Failed' },
    running: { icon: Play, color: 'text-blue-600 dark:text-blue-400', label: 'Running' },
    dry_run: { icon: Eye, color: 'text-violet-600 dark:text-violet-400', label: 'Dry Run' },
    pending: { icon: Clock, color: 'text-amber-600 dark:text-amber-400', label: 'Pending' },
    timed_out: { icon: AlertTriangle, color: 'text-orange-600 dark:text-orange-400', label: 'Timed Out' },
    skipped: { icon: Clock, color: 'text-slate-500 dark:text-slate-400', label: 'Skipped' },
    escalated: { icon: AlertTriangle, color: 'text-red-700 dark:text-red-300', label: 'Escalated' },
}

export default function Remediation() {
    return (
        <div className="space-y-6">
            {/* Header */}
            <div>
                <h1 className="text-2xl font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <Wrench className="h-6 w-6 text-violet-600 dark:text-violet-400" />
                    Auto-Heal
                </h1>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                    When a check fails, the engine runs the remediation command configured on that check, verifies recovery, and auto-resolves the incident. Configure commands per-check on the Checks page.
                </p>
            </div>

            {/* Engine toggle */}
            <EngineToggle />

            {/* Attempt History */}
            <div>
                <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-3">Attempt History</h2>
                <HistoryPanel />
            </div>
        </div>
    )
}

// ─── Engine Toggle (compact banner) ─────────────────────────────────────────

function EngineToggle() {
    const toast = useToast()
    const queryClient = useQueryClient()
    const { data: config, isLoading } = useQuery({
        queryKey: ['remediation-config'],
        queryFn: remediationApi.getConfig,
    })

    const saveMutation = useMutation({
        mutationFn: (cfg: RemediationConfig) => remediationApi.saveConfig(cfg),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['remediation-config'] })
            toast.success('Engine updated')
        },
        onError: () => toast.error('Failed to update engine'),
    })

    if (isLoading || !config) {
        return <div className="text-sm text-slate-500 dark:text-slate-400">Loading…</div>
    }

    const toggleEnabled = () => {
        saveMutation.mutate({ ...config, enabled: !config.enabled })
    }

    return (
        <div className={cn(
            'flex items-center justify-between rounded-lg border p-4',
            config.enabled
                ? 'border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950/30'
                : 'border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-800'
        )}>
            <div className="flex items-center gap-3">
                {config.enabled
                    ? <CheckCircle2 className="h-5 w-5 text-green-600 dark:text-green-400" />
                    : <XCircle className="h-5 w-5 text-slate-400" />}
                <div>
                    <div className={cn(
                        'text-sm font-semibold',
                        config.enabled ? 'text-green-800 dark:text-green-300' : 'text-slate-700 dark:text-slate-300'
                    )}>
                        {config.enabled ? 'Engine is LIVE' : 'Engine is DISABLED'}
                    </div>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        {config.enabled
                            ? 'Remediation commands will run automatically when configured checks fail'
                            : 'Enable to start auto-healing failing checks'}
                    </p>
                </div>
            </div>
            <button onClick={toggleEnabled} className="shrink-0">
                {config.enabled
                    ? <ToggleRight className="h-8 w-8 text-green-600 dark:text-green-400" />
                    : <ToggleLeft className="h-8 w-8 text-slate-400" />}
            </button>
        </div>
    )
}

// ─── History Panel ──────────────────────────────────────────────────────────

function HistoryPanel() {
    const { data, isLoading } = useQuery({
        queryKey: ['remediation-attempts'],
        queryFn: () => remediationApi.listAttempts({ limit: 50 }),
        refetchInterval: 10_000,
    })

    const attempts = data?.attempts ?? []

    if (isLoading) return <div className="text-sm text-slate-500 dark:text-slate-400">Loading history...</div>

    if (attempts.length === 0) {
        return (
            <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 p-8 text-center dark:border-slate-600 dark:bg-slate-800/50">
                <Clock className="mx-auto h-10 w-10 text-slate-400" />
                <h3 className="mt-3 text-sm font-semibold text-slate-700 dark:text-slate-300">No attempts yet</h3>
                <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                    Remediation attempts will appear here when checks fail and the engine triggers actions
                </p>
            </div>
        )
    }

    return (
        <div className="space-y-3">
            {attempts.map((attempt) => (
                <AttemptCard key={attempt.id} attempt={attempt} />
            ))}
        </div>
    )
}

function AttemptCard({ attempt }: { attempt: RemediationAttempt }) {
    const [expanded, setExpanded] = useState(false)
    const status = STATUS_CONFIG[attempt.status] ?? STATUS_CONFIG.pending
    const StatusIcon = status.icon

    return (
        <div className="rounded-lg border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800 overflow-hidden">
            <button
                onClick={() => setExpanded(!expanded)}
                className="w-full flex items-center justify-between gap-3 p-4 text-left"
            >
                <div className="flex items-center gap-3 min-w-0">
                    <StatusIcon className={cn('h-5 w-5 shrink-0', status.color)} />
                    <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                            <span className="text-sm font-semibold text-slate-800 dark:text-slate-200">{attempt.actionName}</span>
                            <span className={cn('text-[10px] px-1.5 py-0.5 rounded font-medium', status.color, 'bg-current/10')}>
                                {status.label}
                            </span>
                            {attempt.dryRun && (
                                <span className="text-[10px] px-1.5 py-0.5 rounded bg-violet-100 text-violet-700 dark:bg-violet-950/40 dark:text-violet-400 font-medium">
                                    DRY RUN
                                </span>
                            )}
                            {attempt.verified === true && (
                                <span className="text-[10px] px-1.5 py-0.5 rounded bg-green-100 text-green-700 dark:bg-green-950/40 dark:text-green-400 font-medium">
                                    Verified
                                </span>
                            )}
                            {attempt.verified === false && (
                                <span className="text-[10px] px-1.5 py-0.5 rounded bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-400 font-medium">
                                    Verify Failed
                                </span>
                            )}
                        </div>
                        <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                            Check: {attempt.checkId} &middot; Attempt #{attempt.attemptNumber} &middot; {attempt.durationMs}ms &middot; Exit {attempt.exitCode} &middot; {new Date(attempt.createdAt).toLocaleString()}
                        </p>
                    </div>
                </div>
                {expanded ? <ChevronUp className="h-4 w-4 text-slate-400 shrink-0" /> : <ChevronDown className="h-4 w-4 text-slate-400 shrink-0" />}
            </button>

            {expanded && (
                <div className="border-t border-slate-100 dark:border-slate-700 p-4 space-y-3">
                    <div className="flex items-center gap-1.5 rounded bg-slate-900 px-3 py-2 text-xs font-mono text-green-400">
                        <span className="text-slate-500">$</span> {attempt.command}
                    </div>
                    {attempt.output && (
                        <div>
                            <span className="text-[10px] font-medium text-slate-500 uppercase">Output</span>
                            <pre className="mt-1 rounded bg-slate-900 p-3 text-xs text-slate-300 overflow-x-auto max-h-40 overflow-y-auto">{attempt.output}</pre>
                        </div>
                    )}
                    {attempt.error && (
                        <div>
                            <span className="text-[10px] font-medium text-red-500 uppercase">Error</span>
                            <pre className="mt-1 rounded bg-red-950/30 border border-red-800/30 p-3 text-xs text-red-300 overflow-x-auto">{attempt.error}</pre>
                        </div>
                    )}
                    {attempt.aiAnalysis && (
                        <div>
                            <span className="text-[10px] font-medium text-violet-500 uppercase flex items-center gap-1">
                                <Sparkles className="h-3 w-3" /> AI Analysis
                            </span>
                            <div className="mt-1 rounded bg-violet-50 border border-violet-200 p-3 text-xs text-violet-800 dark:bg-violet-950/20 dark:border-violet-800 dark:text-violet-300">
                                {attempt.aiAnalysis}
                            </div>
                        </div>
                    )}
                    <div className="text-[10px] text-slate-400 flex gap-4 flex-wrap">
                        <span>Triggered by: {attempt.triggeredBy}</span>
                        <span>Incident: {attempt.incidentId || 'N/A'}</span>
                        <span>Action: {attempt.actionId}</span>
                    </div>
                </div>
            )}
        </div>
    )
}
