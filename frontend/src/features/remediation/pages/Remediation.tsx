import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Wrench,
    Plus,
    Trash2,
    Edit3,
    Terminal,
    Globe,
    Shield,
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
import { remediationApi, type AllowedAction, type RemediationAttempt, type RemediationConfig } from '../api/remediation'
import { useToast } from '@/shared/components/Toast'
import { useConfirm } from '@/shared/components/ConfirmDialog'
import { cn } from '@/shared/lib/utils'

const RISK_STYLES: Record<string, { bg: string; text: string; label: string }> = {
    low: { bg: 'bg-green-50 dark:bg-green-950/30', text: 'text-green-700 dark:text-green-400', label: 'Low' },
    medium: { bg: 'bg-amber-50 dark:bg-amber-950/30', text: 'text-amber-700 dark:text-amber-400', label: 'Medium' },
    high: { bg: 'bg-red-50 dark:bg-red-950/30', text: 'text-red-700 dark:text-red-400', label: 'High' },
}

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

const TYPE_ICONS: Record<string, typeof Terminal> = {
    command: Terminal,
    ssh_command: Terminal,
    http: Globe,
}

type Tab = 'config' | 'actions' | 'history'

export default function Remediation() {
    const [activeTab, setActiveTab] = useState<Tab>('config')

    const tabs: { id: Tab; label: string; count?: number }[] = [
        { id: 'config', label: 'Settings' },
        { id: 'actions', label: 'Action Registry' },
        { id: 'history', label: 'Attempt History' },
    ]

    return (
        <div className="space-y-6">
            {/* Header */}
            <div>
                <h1 className="text-2xl font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <Wrench className="h-6 w-6 text-violet-600 dark:text-violet-400" />
                    Auto-Remediation
                </h1>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                    Automatically heal failing checks with pre-approved actions. When a check fails, the engine executes the linked action, verifies recovery, and auto-resolves the incident.
                </p>
            </div>

            {/* Tabs */}
            <div className="border-b border-slate-200 dark:border-slate-700">
                <div className="flex gap-1">
                    {tabs.map((tab) => (
                        <button
                            key={tab.id}
                            onClick={() => setActiveTab(tab.id)}
                            className={cn(
                                'px-4 py-2.5 text-sm font-medium border-b-2 transition-colors',
                                activeTab === tab.id
                                    ? 'border-violet-600 text-violet-700 dark:border-violet-400 dark:text-violet-300'
                                    : 'border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'
                            )}
                        >
                            {tab.label}
                        </button>
                    ))}
                </div>
            </div>

            {activeTab === 'config' && <ConfigPanel />}
            {activeTab === 'actions' && <ActionsPanel />}
            {activeTab === 'history' && <HistoryPanel />}
        </div>
    )
}

// ─── Config Panel ───────────────────────────────────────────────────────────

function ConfigPanel() {
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
            toast.success('Configuration saved')
        },
        onError: () => toast.error('Failed to save configuration'),
    })

    if (isLoading || !config) {
        return <div className="text-sm text-slate-500 dark:text-slate-400">Loading configuration...</div>
    }

    const toggleEnabled = () => {
        saveMutation.mutate({ ...config, enabled: !config.enabled })
    }

    return (
        <div className="space-y-6">
            {/* Engine toggle */}
            <div className="rounded-lg border border-slate-200 bg-white p-5 dark:border-slate-700 dark:bg-slate-800">
                <div className="flex items-center justify-between">
                    <div>
                        <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-200">Engine Enabled</h3>
                        <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                            When enabled, the engine automatically executes remediation actions on check failures
                        </p>
                    </div>
                    <button onClick={toggleEnabled} className="shrink-0">
                        {config.enabled
                            ? <ToggleRight className="h-8 w-8 text-green-600 dark:text-green-400" />
                            : <ToggleLeft className="h-8 w-8 text-slate-400" />
                        }
                    </button>
                </div>
            </div>

            {/* Status banner */}
            {config.enabled && (
                <div className="flex items-center gap-2 rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950/30 dark:text-green-300">
                    <CheckCircle2 className="h-4 w-4" />
                    <span className="font-medium">LIVE</span> — remediation engine is active and will execute actions automatically
                </div>
            )}
            {!config.enabled && (
                <div className="flex items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400">
                    <XCircle className="h-4 w-4" />
                    <span className="font-medium">DISABLED</span> — enable the engine to start auto-remediating failures
                </div>
            )}

            {/* How it works */}
            <div className="rounded-lg border border-slate-200 bg-white p-5 dark:border-slate-700 dark:bg-slate-800">
                <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-200 mb-3">How Auto-Remediation Works</h3>
                <div className="grid grid-cols-1 sm:grid-cols-4 gap-4">
                    {[
                        { step: '1', title: 'Check Fails', desc: 'A health check detects a failure and creates an incident' },
                        { step: '2', title: 'Action Triggered', desc: 'Engine looks up the linked action and executes it (SSH, local, or HTTP)' },
                        { step: '3', title: 'Verify Recovery', desc: 'After a delay, the check re-runs to confirm the fix worked' },
                        { step: '4', title: 'Auto-Resolve', desc: 'If healthy, the incident auto-resolves and a recovery notification is sent' },
                    ].map((s) => (
                        <div key={s.step} className="text-center">
                            <div className="mx-auto flex h-8 w-8 items-center justify-center rounded-full bg-violet-100 text-sm font-bold text-violet-700 dark:bg-violet-900/50 dark:text-violet-300">
                                {s.step}
                            </div>
                            <h4 className="mt-2 text-xs font-semibold text-slate-700 dark:text-slate-300">{s.title}</h4>
                            <p className="mt-1 text-[11px] text-slate-500 dark:text-slate-400">{s.desc}</p>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    )
}

// ─── Actions Panel ──────────────────────────────────────────────────────────

function ActionsPanel() {
    const toast = useToast()
    const confirm = useConfirm()
    const queryClient = useQueryClient()
    const [showForm, setShowForm] = useState(false)
    const [editingAction, setEditingAction] = useState<AllowedAction | null>(null)

    const { data, isLoading } = useQuery({
        queryKey: ['remediation-actions'],
        queryFn: remediationApi.listActions,
    })

    const deleteMutation = useMutation({
        mutationFn: (id: string) => remediationApi.deleteAction(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['remediation-actions'] })
            toast.success('Action deleted')
        },
        onError: () => toast.error('Failed to delete action'),
    })

    const handleDelete = async (action: AllowedAction) => {
        const ok = await confirm({
            title: 'Delete Action',
            message: `Delete "${action.name}"? Checks referencing this action will stop remediating.`,
            confirmLabel: 'Delete',
            variant: 'danger',
        })
        if (ok) deleteMutation.mutate(action.id)
    }

    const actions = data?.actions ?? []

    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <p className="text-sm text-slate-500 dark:text-slate-400">
                    Pre-approved commands that checks can reference for auto-remediation.
                </p>
                <button
                    onClick={() => { setEditingAction(null); setShowForm(true) }}
                    className="flex items-center gap-1.5 rounded-lg bg-violet-600 px-3 py-2 text-sm font-medium text-white hover:bg-violet-700 transition-colors"
                >
                    <Plus className="h-4 w-4" /> New Action
                </button>
            </div>

            {showForm && (
                <ActionForm
                    initial={editingAction}
                    onClose={() => { setShowForm(false); setEditingAction(null) }}
                />
            )}

            {isLoading ? (
                <div className="text-sm text-slate-500 dark:text-slate-400">Loading actions...</div>
            ) : actions.length === 0 ? (
                <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 p-8 text-center dark:border-slate-600 dark:bg-slate-800/50">
                    <Wrench className="mx-auto h-10 w-10 text-slate-400" />
                    <h3 className="mt-3 text-sm font-semibold text-slate-700 dark:text-slate-300">No actions yet</h3>
                    <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">Create your first remediation action to get started</p>
                </div>
            ) : (
                <div className="grid gap-3">
                    {actions.map((action) => {
                        const risk = RISK_STYLES[action.risk] ?? RISK_STYLES.medium
                        const TypeIcon = TYPE_ICONS[action.type] ?? Terminal
                        return (
                            <div key={action.id} className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-800">
                                <div className="flex items-start justify-between gap-3">
                                    <div className="flex items-start gap-3 min-w-0">
                                        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 dark:bg-slate-700">
                                            <TypeIcon className="h-4 w-4 text-slate-600 dark:text-slate-300" />
                                        </div>
                                        <div className="min-w-0">
                                            <div className="flex items-center gap-2 flex-wrap">
                                                <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-200">{action.name}</h3>
                                                <code className="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400">{action.id}</code>
                                                <span className={cn('inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium', risk.bg, risk.text)}>
                                                    <Shield className="h-2.5 w-2.5" /> {risk.label}
                                                </span>
                                                <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400">
                                                    {action.type}
                                                </span>
                                            </div>
                                            <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">{action.description}</p>
                                            {action.command && (
                                                <div className="mt-2 flex items-center gap-1.5 rounded bg-slate-900 px-3 py-1.5 text-xs font-mono text-green-400">
                                                    <span className="text-slate-500">$</span> {action.command}
                                                </div>
                                            )}
                                            {action.url && (
                                                <div className="mt-2 text-xs text-slate-500 dark:text-slate-400">
                                                    {action.method || 'POST'} {action.url}
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                    <div className="flex items-center gap-1.5 shrink-0">
                                        <button
                                            onClick={() => { setEditingAction(action); setShowForm(true) }}
                                            className="rounded p-1.5 text-slate-400 hover:text-violet-600 hover:bg-violet-50 dark:hover:text-violet-400 dark:hover:bg-violet-950/30"
                                        >
                                            <Edit3 className="h-4 w-4" />
                                        </button>
                                        <button
                                            onClick={() => handleDelete(action)}
                                            className="rounded p-1.5 text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:text-red-400 dark:hover:bg-red-950/30"
                                        >
                                            <Trash2 className="h-4 w-4" />
                                        </button>
                                    </div>
                                </div>
                            </div>
                        )
                    })}
                </div>
            )}
        </div>
    )
}

// ─── Action Form ────────────────────────────────────────────────────────────

function ActionForm({ initial, onClose }: { initial: AllowedAction | null; onClose: () => void }) {
    const toast = useToast()
    const queryClient = useQueryClient()
    const isEdit = initial != null

    const [id, setId] = useState(initial?.id ?? '')
    const [name, setName] = useState(initial?.name ?? '')
    const [type, setType] = useState<string>(initial?.type ?? 'ssh_command')
    const [command, setCommand] = useState(initial?.command ?? '')
    const [url, setUrl] = useState(initial?.url ?? '')
    const [method, setMethod] = useState(initial?.method ?? 'POST')
    const [timeout, setTimeout] = useState(initial?.timeoutSeconds ?? 30)
    const [risk, setRisk] = useState(initial?.risk ?? 'medium')
    const [description, setDescription] = useState(initial?.description ?? '')

    const createMutation = useMutation({
        mutationFn: () => remediationApi.createAction({ id, name, type: type as AllowedAction['type'], command, url, method, timeoutSeconds: timeout, risk: risk as AllowedAction['risk'], description }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['remediation-actions'] })
            toast.success('Action created')
            onClose()
        },
        onError: () => toast.error('Failed to create action'),
    })

    const updateMutation = useMutation({
        mutationFn: () => remediationApi.updateAction(initial!.id, { name, type: type as AllowedAction['type'], command, url, method, timeoutSeconds: timeout, risk: risk as AllowedAction['risk'], description }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['remediation-actions'] })
            toast.success('Action updated')
            onClose()
        },
        onError: () => toast.error('Failed to update action'),
    })

    const inputCls = 'w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-white'
    const labelCls = 'block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1'

    return (
        <div className="rounded-lg border border-violet-200 bg-violet-50/50 p-5 dark:border-violet-800 dark:bg-violet-950/20">
            <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-200 mb-4">
                {isEdit ? 'Edit Action' : 'New Remediation Action'}
            </h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                    <label className={labelCls}>Action ID</label>
                    <input className={inputCls} value={id} onChange={(e) => setId(e.target.value)} placeholder="restart-nginx" disabled={isEdit} />
                </div>
                <div>
                    <label className={labelCls}>Name</label>
                    <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="Restart Nginx" />
                </div>
                <div>
                    <label className={labelCls}>Type</label>
                    <select className={inputCls} value={type} onChange={(e) => setType(e.target.value)}>
                        <option value="ssh_command">SSH Command</option>
                        <option value="command">Local Command</option>
                        <option value="http">HTTP Webhook</option>
                    </select>
                </div>
                <div>
                    <label className={labelCls}>Risk Level</label>
                    <select className={inputCls} value={risk} onChange={(e) => setRisk(e.target.value as 'low' | 'medium' | 'high')}>
                        <option value="low">Low</option>
                        <option value="medium">Medium</option>
                        <option value="high">High</option>
                    </select>
                </div>
                {type !== 'http' && (
                    <div className="md:col-span-2">
                        <label className={labelCls}>Command</label>
                        <input className={cn(inputCls, 'font-mono')} value={command} onChange={(e) => setCommand(e.target.value)} placeholder="sudo systemctl restart nginx" />
                    </div>
                )}
                {type === 'http' && (
                    <>
                        <div>
                            <label className={labelCls}>URL</label>
                            <input className={inputCls} value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://api.example.com/restart" />
                        </div>
                        <div>
                            <label className={labelCls}>Method</label>
                            <select className={inputCls} value={method} onChange={(e) => setMethod(e.target.value)}>
                                <option value="POST">POST</option>
                                <option value="PUT">PUT</option>
                                <option value="GET">GET</option>
                            </select>
                        </div>
                    </>
                )}
                <div>
                    <label className={labelCls}>Timeout (seconds)</label>
                    <input className={inputCls} type="number" value={timeout} onChange={(e) => setTimeout(Number(e.target.value))} min={1} max={300} />
                </div>
                <div>
                    <label className={labelCls}>Description</label>
                    <input className={inputCls} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Restarts the nginx service via systemd" />
                </div>
            </div>
            <div className="flex items-center gap-2 mt-4">
                <button
                    onClick={() => isEdit ? updateMutation.mutate() : createMutation.mutate()}
                    disabled={createMutation.isPending || updateMutation.isPending}
                    className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white hover:bg-violet-700 disabled:opacity-50"
                >
                    {isEdit ? 'Update' : 'Create'} Action
                </button>
                <button onClick={onClose} className="rounded-lg px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-700">
                    Cancel
                </button>
            </div>
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
