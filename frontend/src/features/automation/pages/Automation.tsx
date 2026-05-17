import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Zap,
    Play,
    XCircle,
    Clock,
    CheckCircle2,
    AlertTriangle,
    Shield,
    Terminal,
    RefreshCw,
    Sparkles,
    FileText,
    ChevronDown,
    ChevronUp,
} from 'lucide-react'
import { automationApi, type AutomationAction, type AuditEntry } from '../api/automation'
import { useConfirm } from '@/shared/components/ConfirmDialog'
import { useToast } from '@/shared/components/Toast'
import { useAuth } from '@/shared/hooks/useAuth'
import { APIError } from '@/shared/api/client'
import { useAIAvailability } from '@/features/ai/hooks/useAIAvailability'

const RISK_STYLES: Record<string, { bg: string; text: string; label: string }> = {
    low: { bg: 'bg-green-50 dark:bg-green-950/30', text: 'text-green-700 dark:text-green-400', label: 'Low Risk' },
    medium: { bg: 'bg-amber-50 dark:bg-amber-950/30', text: 'text-amber-700 dark:text-amber-400', label: 'Medium Risk' },
    high: { bg: 'bg-orange-50 dark:bg-orange-950/30', text: 'text-orange-700 dark:text-orange-400', label: 'High Risk' },
    critical: { bg: 'bg-red-50 dark:bg-red-950/30', text: 'text-red-700 dark:text-red-400', label: 'Critical Risk' },
}

const STATUS_ICONS: Record<string, typeof CheckCircle2> = {
    pending: Clock,
    approved: CheckCircle2,
    rejected: XCircle,
    expired: AlertTriangle,
}

const STATUS_STYLES: Record<string, string> = {
    pending: 'text-amber-600 bg-amber-50 dark:text-amber-400 dark:bg-amber-950/30',
    approved: 'text-green-600 bg-green-50 dark:text-green-400 dark:bg-green-950/30',
    rejected: 'text-red-600 bg-red-50 dark:text-red-400 dark:bg-red-950/30',
    expired: 'text-slate-500 bg-slate-50 dark:text-slate-400 dark:bg-slate-800',
}

const AUTOMATION_STEPS = [
    { icon: Sparkles, title: 'Generate', text: 'AI reads current incidents, checks, and any context you add.' },
    { icon: Shield, title: 'Review', text: 'Each suggestion is labeled by risk and shows the exact command, if one exists.' },
    { icon: FileText, title: 'Record', text: 'Approve or reject stores an audit decision. HealthOps does not run commands.' },
]

function ActionCard({
    action,
    onApprove,
    onReject,
    isMutating,
}: {
    action: AutomationAction
    onApprove: (action: AutomationAction) => void
    onReject: (action: AutomationAction) => void
    isMutating: boolean
}) {
    const [expanded, setExpanded] = useState(false)
    const risk = RISK_STYLES[action.risk] ?? RISK_STYLES.low
    const StatusIcon = STATUS_ICONS[action.status] ?? Clock

    return (
        <div className="rounded-lg border border-slate-200 bg-white p-4 transition-shadow hover:shadow-md dark:border-slate-700 dark:bg-slate-800">
            <div className="flex items-start justify-between gap-3">
                <div className="flex items-start gap-3">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-violet-50 dark:bg-violet-950/40">
                        <Zap className="h-4.5 w-4.5 text-violet-600 dark:text-violet-400" />
                    </div>
                    <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                            <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-200">{action.title}</h3>
                            <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium ${risk.bg} ${risk.text}`}>
                                <Shield className="h-2.5 w-2.5" />
                                {risk.label}
                            </span>
                            <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium ${STATUS_STYLES[action.status]}`}>
                                <StatusIcon className="h-2.5 w-2.5" />
                                {action.status}
                            </span>
                            <span className="inline-flex items-center gap-1 rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-600 dark:bg-slate-700 dark:text-slate-300">
                                <FileText className="h-2.5 w-2.5" />
                                Review only
                            </span>
                        </div>
                        <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">{action.description}</p>
                        <p className="mt-1 text-xs text-slate-500">
                            <span className="font-medium">Why:</span> {action.reason}
                        </p>
                    </div>
                </div>
                <button
                    onClick={() => setExpanded(!expanded)}
                    className="rounded p-1 text-slate-400 transition-colors hover:bg-slate-100 dark:hover:bg-slate-700"
                >
                    {expanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                </button>
            </div>

            {expanded && (
                <div className="mt-3 space-y-2 border-t border-slate-100 pt-3 dark:border-slate-700">
                    {action.command && (
                        <div className="rounded bg-slate-900 p-2.5">
                            <div className="flex items-center gap-1.5 text-[10px] text-slate-400">
                                <Terminal className="h-3 w-3" />
                                Suggested Command
                            </div>
                            <code className="mt-1 block text-xs text-green-400 font-mono">{action.command}</code>
                            <p className="mt-2 text-xs text-slate-400">
                                Run manually only after confirming the host, scope, and rollback path.
                            </p>
                        </div>
                    )}
                    <div className="flex items-center gap-2 text-xs text-slate-500">
                        <Clock className="h-3 w-3" />
                        Expires: {new Date(action.expiresAt).toLocaleString()}
                    </div>
                </div>
            )}

            {action.status === 'pending' && (
                <div className="mt-3 flex items-center gap-2 border-t border-slate-100 pt-3 dark:border-slate-700">
                    <button
                        onClick={() => onApprove(action)}
                        disabled={isMutating}
                        className="flex items-center gap-1.5 rounded-lg bg-green-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-green-700 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                        <Play className="h-3 w-3" />
                        Mark approved
                    </button>
                    <button
                        onClick={() => onReject(action)}
                        disabled={isMutating}
                        className="flex items-center gap-1.5 rounded-lg border border-red-200 px-3 py-1.5 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950/30"
                    >
                        <XCircle className="h-3 w-3" />
                        Reject
                    </button>
                </div>
            )}
        </div>
    )
}

function AuditTable({ entries }: { entries: AuditEntry[] }) {
    if (entries.length === 0) {
        return <p className="py-4 text-center text-xs text-slate-500">No audit entries yet.</p>
    }
    return (
        <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-700">
            <table className="w-full text-xs">
                <thead className="bg-slate-50 dark:bg-slate-800">
                    <tr>
                        <th className="px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Time</th>
                        <th className="px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Event</th>
                        <th className="px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Actor</th>
                        <th className="px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Details</th>
                    </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                    {entries.slice(0, 50).map((entry) => (
                        <tr key={entry.id} className="text-slate-700 dark:text-slate-300">
                            <td className="px-3 py-2 whitespace-nowrap">{new Date(entry.timestamp).toLocaleString()}</td>
                            <td className="px-3 py-2">
                                <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${entry.event === 'approved' ? 'bg-green-50 text-green-700 dark:bg-green-950/30 dark:text-green-400' :
                                    entry.event === 'rejected' ? 'bg-red-50 text-red-700 dark:bg-red-950/30 dark:text-red-400' :
                                        'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'
                                    }`}>
                                    {entry.event}
                                </span>
                            </td>
                            <td className="px-3 py-2">{entry.actor}</td>
                            <td className="px-3 py-2 text-slate-500">{entry.details || '—'}</td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    )
}

export default function Automation() {
    const [tab, setTab] = useState<'actions' | 'audit'>('actions')
    const [statusFilter, setStatusFilter] = useState('')
    const [suggestContext, setSuggestContext] = useState('')
    const queryClient = useQueryClient()
    const confirm = useConfirm()
    const toast = useToast()
    const { user } = useAuth()
    const { isAIAvailable } = useAIAvailability()

    const { data: status } = useQuery({
        queryKey: ['automation', 'status'],
        queryFn: automationApi.status,
    })

    const { data: actionsData, isLoading } = useQuery({
        queryKey: ['automation', 'actions', statusFilter],
        queryFn: () => automationApi.listActions(statusFilter || undefined),
    })

    const { data: auditData } = useQuery({
        queryKey: ['automation', 'audit'],
        queryFn: automationApi.audit,
        enabled: tab === 'audit',
    })

    const suggestMutation = useMutation({
        mutationFn: () => automationApi.suggest(undefined, undefined, suggestContext),
        onSuccess: (result) => {
            queryClient.invalidateQueries({ queryKey: ['automation'] })
            if ((result.actions ?? []).length === 0) {
                toast.info('No remediation suggestions were generated. Add more context or open an incident first.')
                return
            }
            setSuggestContext('')
            toast.success(`Generated ${result.actions.length} remediation ${result.actions.length === 1 ? 'suggestion' : 'suggestions'}`)
        },
        onError: (err: Error) => toast.error(err.message || 'Failed to generate remediation suggestions'),
    })

    const approveMutation = useMutation({
        mutationFn: (id: string) => automationApi.approve(id, user?.username || 'unknown'),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['automation'] })
            toast.success('Suggestion approved in audit log')
        },
        onError: (err: Error) => {
            queryClient.invalidateQueries({ queryKey: ['automation'] })
            if (isStaleActionError(err)) {
                toast.warning('This suggestion is no longer available. The action list has been refreshed.')
                return
            }
            toast.error(err.message || 'Failed to approve action')
        },
    })

    const rejectMutation = useMutation({
        mutationFn: (id: string) => automationApi.reject(id, user?.username || 'unknown'),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['automation'] })
            toast.success('Action rejected')
        },
        onError: (err: Error) => {
            queryClient.invalidateQueries({ queryKey: ['automation'] })
            if (isStaleActionError(err)) {
                toast.warning('This suggestion is no longer available. The action list has been refreshed.')
                return
            }
            toast.error(err.message || 'Failed to reject action')
        },
    })

    const actions = actionsData?.actions ?? []
    const isAvailable = status?.enabled ?? false
    const isActionMutating = approveMutation.isPending || rejectMutation.isPending

    if (!isAIAvailable) return null

    const handleSuggest = () => {
        if (suggestMutation.isPending) return
        suggestMutation.mutate()
    }

    const handleApprove = async (action: AutomationAction) => {
        const ok = await confirm({
            title: 'Approve remediation suggestion?',
            message: `Approve "${action.title}" as a reviewed recommendation only. HealthOps will not execute the command automatically.`,
            confirmLabel: 'Mark approved',
        })
        if (ok) approveMutation.mutate(action.id)
    }

    const handleReject = async (action: AutomationAction) => {
        const ok = await confirm({
            title: 'Reject remediation suggestion?',
            message: `Reject "${action.title}" and keep the decision in the audit log.`,
            confirmLabel: 'Reject',
            variant: 'danger',
        })
        if (ok) rejectMutation.mutate(action.id)
    }

    return (
        <div className="flex h-full flex-col">
            {/* Header */}
            <div className="border-b border-slate-200 px-6 py-4 dark:border-slate-700">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-600 text-white">
                            <Zap className="h-5 w-5" />
                        </div>
                        <div>
                            <div className="flex flex-wrap items-center gap-2">
                                <h1 className="text-lg font-bold text-slate-800 dark:text-white">Remediation Suggestions</h1>
                                <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300">
                                    <Shield className="h-3 w-3" />
                                    Manual execution
                                </span>
                            </div>
                            <p className="text-xs text-slate-500">AI drafts operator actions from monitoring context; approvals are audit records, not automatic infrastructure changes.</p>
                        </div>
                    </div>
                    {!isAvailable && (
                        <span className="rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-500 dark:bg-slate-800">
                            Suggestions unavailable
                        </span>
                    )}
                </div>

                {/* Tabs */}
                <div className="mt-3 flex items-center gap-4">
                    <button
                        onClick={() => setTab('actions')}
                        className={`flex items-center gap-1.5 border-b-2 pb-1 text-xs font-medium transition-colors ${tab === 'actions'
                            ? 'border-violet-600 text-violet-600 dark:border-violet-400 dark:text-violet-400'
                            : 'border-transparent text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
                            }`}
                    >
                        <Zap className="h-3.5 w-3.5" />
                        Actions
                    </button>
                    <button
                        onClick={() => setTab('audit')}
                        className={`flex items-center gap-1.5 border-b-2 pb-1 text-xs font-medium transition-colors ${tab === 'audit'
                            ? 'border-violet-600 text-violet-600 dark:border-violet-400 dark:text-violet-400'
                            : 'border-transparent text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
                            }`}
                    >
                        <FileText className="h-3.5 w-3.5" />
                        Audit Log
                    </button>
                </div>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-5">
                {tab === 'actions' && (
                    <div className="space-y-4">
                        {/* Suggest form */}
                        {isAvailable && (
                            <div className="flex items-center gap-2">
                                <input
                                    type="text"
                                    value={suggestContext}
                                    onChange={(e) => setSuggestContext(e.target.value)}
                                    placeholder="Optional context, e.g. checkout is slow after the Mongo restart"
                                    className="flex-1 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 placeholder-slate-400 outline-none focus:border-violet-400 focus:ring-2 focus:ring-violet-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                                />
                                <button
                                    onClick={handleSuggest}
                                    disabled={suggestMutation.isPending}
                                    className="flex items-center gap-1.5 rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-violet-700 disabled:cursor-not-allowed disabled:opacity-50"
                                >
                                    {suggestMutation.isPending ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                                    {suggestMutation.isPending ? 'Generating...' : 'Generate Suggestions'}
                                </button>
                            </div>
                        )}
                        <div className="grid gap-3 md:grid-cols-3">
                            {AUTOMATION_STEPS.map((step) => {
                                const Icon = step.icon
                                return (
                                    <div key={step.title} className="rounded-lg border border-slate-200 bg-white px-3 py-3 dark:border-slate-700 dark:bg-slate-800">
                                        <div className="flex items-center gap-2 text-xs font-semibold text-slate-800 dark:text-slate-100">
                                            <Icon className="h-3.5 w-3.5 text-violet-600 dark:text-violet-400" />
                                            {step.title}
                                        </div>
                                        <p className="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{step.text}</p>
                                    </div>
                                )
                            })}
                        </div>
                        <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300">
                            Treat suggestions like runbook drafts. Verify the target, blast radius, and rollback path before running commands outside HealthOps.
                        </div>

                        {/* Filter */}
                        <div className="flex gap-1">
                            {[
                                { value: '', label: 'All' },
                                { value: 'pending', label: 'Pending' },
                                { value: 'approved', label: 'Approved' },
                                { value: 'rejected', label: 'Rejected' },
                            ].map((f) => (
                                <button
                                    key={f.value}
                                    onClick={() => setStatusFilter(f.value)}
                                    className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${statusFilter === f.value
                                        ? 'bg-slate-800 text-white dark:bg-slate-200 dark:text-slate-900'
                                        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800'
                                        }`}
                                >
                                    {f.label}
                                </button>
                            ))}
                        </div>

                        {/* Actions list */}
                        {isLoading ? (
                            <div className="flex justify-center py-12">
                                <RefreshCw className="h-5 w-5 animate-spin text-slate-400" />
                            </div>
                        ) : suggestMutation.isPending ? (
                            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-violet-200 bg-violet-50/40 py-16 text-center dark:border-violet-900/60 dark:bg-violet-950/20">
                                <RefreshCw className="h-6 w-6 animate-spin text-violet-600 dark:text-violet-400" />
                                <p className="mt-3 text-sm font-medium text-slate-700 dark:text-slate-300">Generating remediation suggestions...</p>
                                <p className="mt-1 max-w-md text-xs text-slate-500">The AI is reading current telemetry and incident context. Suggested commands will still require manual review.</p>
                            </div>
                        ) : actions.length === 0 ? (
                            <div className="flex flex-col items-center py-16 text-center">
                                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-slate-50 dark:bg-slate-800">
                                    <Zap className="h-6 w-6 text-slate-400" />
                                </div>
                                <p className="mt-3 text-sm font-medium text-slate-700 dark:text-slate-300">No remediation suggestions yet</p>
                                <p className="mt-1 max-w-md text-xs leading-5 text-slate-500">
                                    Generate suggestions after an incident opens, or add context about the symptom you are investigating.
                                </p>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                {actions.map((action) => (
                                    <ActionCard
                                        key={action.id}
                                        action={action}
                                        onApprove={handleApprove}
                                        onReject={handleReject}
                                        isMutating={isActionMutating}
                                    />
                                ))}
                            </div>
                        )}
                    </div>
                )}

                {tab === 'audit' && (
                    <AuditTable entries={auditData?.entries ?? []} />
                )}
            </div>
        </div>
    )
}

function isStaleActionError(err: Error) {
    return (err instanceof APIError && err.code === 404) || /action .* not found/i.test(err.message)
}
