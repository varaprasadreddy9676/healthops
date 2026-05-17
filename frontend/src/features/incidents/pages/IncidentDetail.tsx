import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, CheckCircle, Eye, Brain, ChevronDown, ShieldAlert, Activity } from 'lucide-react'
import { useState } from 'react'
import { incidentsApi } from "@/features/incidents/api/incidents"
import { aiApi } from "@/features/ai/api/ai"
import { useAIAvailability } from "@/features/ai/hooks/useAIAvailability"
import { checksApi } from "@/features/checks/api/checks"
import { RCAPanel } from "@/features/incidents/components/RCAPanel"
import { EvidenceBriefCard } from "@/features/incidents/components/EvidenceBriefCard"
import { EvidenceTimeline } from "@/features/incidents/components/EvidenceTimeline"
import { MarkdownContent } from "@/shared/components/MarkdownContent"
import { LoadingState } from "@/shared/components/LoadingState"
import { ErrorState } from "@/shared/components/ErrorState"
import { useConfirm } from "@/shared/components/ConfirmDialog"
import { useToast } from "@/shared/components/Toast"
import { useAuth } from "@/shared/hooks/useAuth"
import { cn, relativeTime, formatDate, formatDuration, incidentStatusLabel, severityColor, incidentMessageSummary, checkTypeLabel } from "@/shared/lib/utils"
import type { CheckConfig, CheckDetail as CheckDetailType, Incident } from "@/shared/types"

export default function IncidentDetail() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const confirm = useConfirm()
  const toast = useToast()
  const { user } = useAuth()
  const { isAIAvailable: aiEnabled } = useAIAvailability()
  const actor = user?.displayName || user?.username || 'operator'

  const { data: incident, isLoading, error, refetch } = useQuery({
    queryKey: ['incidents', id],
    queryFn: () => incidentsApi.get(id!),
    enabled: !!id,
  })

  const incidentCheckId = incident?.checkId

  const { data: checkDetail } = useQuery({
    queryKey: ['checks', incidentCheckId],
    queryFn: () => checksApi.get(incidentCheckId!),
    enabled: !!incidentCheckId,
    retry: false,
  })

  const { data: snapshots } = useQuery({
    queryKey: ['incidents', id, 'snapshots'],
    queryFn: () => incidentsApi.snapshots(id!),
    enabled: !!id,
  })

  const { data: aiResults } = useQuery({
    queryKey: ['ai', 'results', id],
    queryFn: () => aiApi.results(id!),
    enabled: !!id && aiEnabled,
    retry: false,
  })

  const ackMutation = useMutation({
    mutationFn: () => incidentsApi.acknowledge(id!, actor),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['incidents'] })
      toast.success('Incident acknowledged')
    },
    onError: (err: Error) => toast.error(err.message || 'Failed to acknowledge incident'),
  })

  const resolveMutation = useMutation({
    mutationFn: () => incidentsApi.resolve(id!, actor),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['incidents'] })
      toast.success('Incident resolved')
    },
    onError: (err: Error) => toast.error(err.message || 'Failed to resolve incident'),
  })

  const analyzeMutation = useMutation({
    mutationFn: () => aiApi.analyze(id!),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['ai', 'results', id] }) },
    onError: (err: Error) => toast.error(err.message || 'AI analysis failed'),
  })

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState message={error.message} retry={() => refetch()} />
  if (!incident) return null
  const aiResult = aiResults?.[0]

  const handleAcknowledge = async () => {
    const ok = await confirm({
      title: 'Acknowledge Incident',
      message: `Acknowledge "${incident.checkName}" as ${actor}? This keeps the incident open but records operator ownership.`,
      confirmLabel: 'Acknowledge',
    })
    if (ok) ackMutation.mutate()
  }

  const handleResolve = async () => {
    const ok = await confirm({
      title: 'Resolve Incident',
      message: `Resolve "${incident.checkName}" as ${actor}? Use this only after the service has recovered or the alert is confirmed closed.`,
      confirmLabel: 'Resolve',
    })
    if (ok) resolveMutation.mutate()
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-start gap-3">
        <Link to="/incidents" className="mt-1 rounded-md p-1 text-slate-400 transition-colors hover:text-slate-600">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div className="flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">{incident.checkName}</h1>
            <span className={cn(
              'rounded-full px-2.5 py-0.5 text-xs font-semibold uppercase',
              incident.status === 'open' ? 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-400' :
                incident.status === 'acknowledged' ? 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400' :
                  'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400'
            )}>
              {incidentStatusLabel(incident.status)}
            </span>
            <span className={cn('text-sm font-medium capitalize', severityColor(incident.severity))}>
              {incident.severity}
            </span>
          </div>
          <p className="mt-1 text-sm text-slate-500">{incidentMessageSummary(incident.message)}</p>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
        <IncidentEvidenceCard incident={incident} checkDetail={checkDetail} />
        <OperatorActionsCard
          incident={incident}
          acknowledgePending={ackMutation.isPending}
          resolvePending={resolveMutation.isPending}
          onAcknowledge={handleAcknowledge}
          onResolve={handleResolve}
        />
      </div>

      {/* Timeline */}
      <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
        <h2 className="mb-4 text-sm font-semibold text-slate-900 dark:text-slate-100">Incident lifecycle</h2>
        <div className="space-y-4">
          <TimelineEntry
            label="Started"
            time={incident.startedAt}
            detail={incidentMessageSummary(incident.message)}
            color="bg-red-500"
          />
          {incident.acknowledgedAt && (
            <TimelineEntry
              label="Acknowledged"
              time={incident.acknowledgedAt}
              detail={incident.acknowledgedBy ? `by ${incident.acknowledgedBy}` : undefined}
              color="bg-amber-500"
            />
          )}
          {incident.resolvedAt && (
            <TimelineEntry
              label="Resolved"
              time={incident.resolvedAt}
              detail={incident.resolvedBy ? `by ${incident.resolvedBy}` : undefined}
              color="bg-emerald-500"
            />
          )}
        </div>
      </div>

      {/* Evidence Timeline */}
      <EvidenceTimeline incidentId={id!} showAIEvents={aiEnabled} />

      {/* Evidence snapshots */}
      {snapshots && snapshots.length > 0 && (
        <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
          <h2 className="mb-3 text-sm font-semibold text-slate-900 dark:text-slate-100">
            Evidence Snapshots ({snapshots.length})
          </h2>
          <div className="space-y-3">
            {snapshots.map((snap, i) => (
              <details key={i} className="group rounded-lg bg-slate-50 dark:bg-slate-800/50">
                <summary className="cursor-pointer px-4 py-2.5 text-sm font-medium text-slate-700 dark:text-slate-300">
                  {snap.snapshotType} — {formatDate(snap.timestamp)}
                </summary>
                <pre className="overflow-x-auto border-t border-slate-200 px-4 py-3 font-mono text-xs text-slate-600 dark:border-slate-700 dark:text-slate-400">
                  {(() => { try { return JSON.stringify(JSON.parse(snap.payloadJson), null, 2) } catch { return snap.payloadJson } })()}
                </pre>
              </details>
            ))}
          </div>
        </div>
      )}

      {aiEnabled && (
        <details className="group rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-4 px-5 py-4">
            <div>
              <div className="flex items-center gap-2">
                <Brain className="h-4 w-4 text-slate-500" />
                <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">AI-assisted investigation</h2>
              </div>
              <p className="mt-1 text-xs text-slate-500">
                Optional analysis and correlation. Verify these suggestions against the evidence above before acting.
              </p>
            </div>
            <ChevronDown className="h-4 w-4 shrink-0 text-slate-400 transition-transform group-open:rotate-180" />
          </summary>
          <div className="space-y-4 border-t border-slate-100 p-5 dark:border-slate-800">
            <button
              onClick={() => analyzeMutation.mutate()}
              disabled={analyzeMutation.isPending}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800"
            >
              <Brain className="h-3.5 w-3.5" />
              {analyzeMutation.isPending ? 'Analyzing...' : 'Run AI analysis'}
            </button>
            <EvidenceBriefCard incidentId={id!} />
            {aiResult && <AIAnalysisCard aiResult={aiResult} />}
            <RCAPanel incidentId={id!} aiEnabled={aiEnabled} />
          </div>
        </details>
      )}

      {/* Metadata */}
      {incident.metadata && Object.keys(incident.metadata).length > 0 && (
        <details className="group rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
          <summary className="flex cursor-pointer list-none items-center justify-between px-5 py-4">
            <div>
              <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">Raw metadata</h2>
              <p className="mt-1 text-xs text-slate-500">Internal alert fields for debugging integrations and exports.</p>
            </div>
            <ChevronDown className="h-4 w-4 text-slate-400 transition-transform group-open:rotate-180" />
          </summary>
          <dl className="grid gap-3 border-t border-slate-100 p-5 sm:grid-cols-2 dark:border-slate-800">
            {Object.entries(incident.metadata).map(([k, v]) => (
              <div key={k} className="min-w-0 rounded-lg bg-slate-50 px-3 py-2 text-sm dark:bg-slate-950/40">
                <dt className="text-[10px] font-semibold uppercase tracking-wide text-slate-400">{k}</dt>
                <dd className="mt-1 break-words text-slate-700 dark:text-slate-300">{v}</dd>
              </div>
            ))}
          </dl>
        </details>
      )}
    </div>
  )
}

function IncidentEvidenceCard({ incident, checkDetail }: { incident: Incident; checkDetail?: CheckDetailType }) {
  const config = checkDetail?.config
  const latestResult = checkDetail?.latestResult
  const source = formatCheckSource(config, incident)
  const staleSeconds = parseStaleSeconds(latestResult?.message) ?? parseStaleSeconds(incident.message)
  const freshnessSeconds = config?.freshnessSeconds
  const latestFinishedAt = latestResult?.finishedAt
  const lastObservedAt = staleSeconds && latestFinishedAt
    ? new Date(new Date(latestFinishedAt).getTime() - staleSeconds * 1000)
    : undefined
  const realError = incidentMessageSummary(latestResult?.message || incident.message)

  return (
    <section className="rounded-xl border border-red-200 bg-white p-5 dark:border-red-900/60 dark:bg-slate-900">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-red-600 dark:text-red-400">Current evidence</p>
          <h2 className="mt-1 text-lg font-semibold text-slate-950 dark:text-slate-50">{realError}</h2>
          <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-600 dark:text-slate-400">
            {incidentEvidenceExplanation(config, staleSeconds, freshnessSeconds)}
          </p>
        </div>
        {latestResult && (
          <span className={cn(
            'shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold uppercase',
            latestResult.status === 'critical' ? 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-400' :
              latestResult.status === 'warning' ? 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400' :
                latestResult.status === 'healthy' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400' :
                  'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
          )}>
            Latest check: {latestResult.status}
          </span>
        )}
      </div>

      <dl className="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <EvidenceFact label="Monitor" value={config?.name || incident.checkName} />
        <EvidenceFact label="Type" value={checkTypeLabel(config?.type || incident.type || 'check')} />
        <EvidenceFact label="Source" value={source} />
        <EvidenceFact label="Server / app" value={[config?.server || latestResult?.server, config?.application || latestResult?.application].filter(Boolean).join(' / ') || 'Not specified'} />
        {freshnessSeconds && <EvidenceFact label="Expected freshness" value={`New log activity within ${formatDuration(freshnessSeconds * 1000)}`} />}
        {staleSeconds && <EvidenceFact label="Observed staleness" value={`No update for ${formatDuration(staleSeconds * 1000)}`} />}
        {lastObservedAt && Number.isFinite(lastObservedAt.getTime()) && (
          <EvidenceFact label="Approx. last update" value={formatDate(lastObservedAt.toISOString(), 'MMM d, HH:mm:ss')} />
        )}
        {latestFinishedAt && (
          <EvidenceFact label="Latest run" value={`${relativeTime(latestFinishedAt)} (${formatDate(latestFinishedAt, 'MMM d, HH:mm:ss')})`} />
        )}
      </dl>

      {config?.type === 'log' && (
        <div className="mt-5 rounded-lg border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950/40">
          <p className="text-sm font-medium text-slate-900 dark:text-slate-100">Why a stale log matters</p>
          <p className="mt-1 text-sm leading-6 text-slate-600 dark:text-slate-400">
            This alert means the watched log source stopped changing. Common causes are a stopped worker, a stuck logger,
            rotated logs writing to a different file, a missing container mount, or permissions preventing writes.
            It is a log-pipeline symptom first; use the source path and latest run above to confirm the actual service impact.
          </p>
        </div>
      )}
    </section>
  )
}

function OperatorActionsCard({
  incident,
  acknowledgePending,
  resolvePending,
  onAcknowledge,
  onResolve,
}: {
  incident: Incident
  acknowledgePending: boolean
  resolvePending: boolean
  onAcknowledge: () => void
  onResolve: () => void
}) {
  return (
    <aside className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
      <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">Operator actions</h2>
      <p className="mt-1 text-xs leading-5 text-slate-500">
        Acknowledge ownership, then resolve only after the latest check is healthy or the alert is confirmed closed.
      </p>
      {incident.status !== 'resolved' ? (
        <div className="mt-4 flex flex-col gap-2">
          {incident.status === 'open' && (
            <button
              onClick={onAcknowledge}
              disabled={acknowledgePending}
              className="inline-flex items-center justify-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2 text-sm font-medium text-amber-700 transition-colors hover:bg-amber-100 disabled:opacity-50 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400"
            >
              <Eye className="h-4 w-4" />
              Acknowledge
            </button>
          )}
          <button
            onClick={onResolve}
            disabled={resolvePending}
            className="inline-flex items-center justify-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm font-medium text-emerald-700 transition-colors hover:bg-emerald-100 disabled:opacity-50 dark:border-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-400"
          >
            <CheckCircle className="h-4 w-4" />
            Resolve
          </button>
        </div>
      ) : (
        <p className="mt-4 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-400">
          Resolved{incident.resolvedBy ? ` by ${incident.resolvedBy}` : ''}.
        </p>
      )}
    </aside>
  )
}

function EvidenceFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2.5 dark:border-slate-800 dark:bg-slate-950/40">
      <dt className="text-[10px] font-semibold uppercase tracking-wide text-slate-400">{label}</dt>
      <dd className="mt-1 break-words text-sm font-medium text-slate-800 dark:text-slate-200">{value}</dd>
    </div>
  )
}

function parseStaleSeconds(message?: string): number | undefined {
  const match = message?.match(/log heartbeat stale:\s*last update\s*(\d+)s\s*ago/i)
  if (!match) return undefined
  const value = Number(match[1])
  return Number.isFinite(value) ? value : undefined
}

function formatCheckSource(config: CheckConfig | undefined, incident: Incident): string {
  if (!config) return incident.checkId
  if (config.type === 'log' && config.path) return config.path
  if (config.type === 'process' && config.target) return config.target
  if (config.type === 'process' && config.command) return config.command
  if (config.type === 'api' && config.target) return config.target
  if (config.host && config.port) return `${config.host}:${config.port}`
  if (config.host) return config.host
  if (config.target) return config.target
  if (config.command) return config.command
  return config.id
}

function incidentEvidenceExplanation(config: CheckConfig | undefined, staleSeconds?: number, freshnessSeconds?: number): string {
  if (config?.type === 'log') {
    const threshold = freshnessSeconds ? formatDuration(freshnessSeconds * 1000) : 'the configured freshness window'
    const observed = staleSeconds ? `; the latest run saw no update for ${formatDuration(staleSeconds * 1000)}` : ''
    return `HealthOps expects this log source to receive fresh writes within ${threshold}${observed}.`
  }
  if (config?.type === 'heartbeat') {
    return 'HealthOps expected an external heartbeat ping but has not received one inside the configured interval.'
  }
  if (config?.type === 'process') {
    return 'HealthOps checked for the configured process on the target host and did not find a matching running process.'
  }
  return 'HealthOps opened this incident from the latest failed monitor result. Use the facts below before relying on correlation or suggested actions.'
}

function AIAnalysisCard({ aiResult }: { aiResult: { summary?: string; analysis: string; suggestions?: string[]; severity?: string; confidence?: string; createdAt: string } }) {
  const [expanded, setExpanded] = useState(false)

  const confidenceColor = aiResult.confidence === 'high'
    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400'
    : aiResult.confidence === 'medium'
      ? 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400'
      : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'

  const severityBadge = aiResult.severity === 'critical'
    ? 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-400'
    : aiResult.severity === 'high'
      ? 'bg-orange-100 text-orange-700 dark:bg-orange-950/40 dark:text-orange-400'
      : aiResult.severity === 'medium'
        ? 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400'
        : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
      {/* Header */}
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Brain className="h-4 w-4 text-slate-500" />
          <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">Previous AI analysis</h2>
        </div>
        <div className="flex items-center gap-2">
          {aiResult.confidence && (
            <span className={cn('rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase', confidenceColor)}>
              {aiResult.confidence} confidence
            </span>
          )}
          {aiResult.severity && (
            <span className={cn('rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase', severityBadge)}>
              <ShieldAlert className="mr-0.5 inline h-3 w-3" />
              {aiResult.severity}
            </span>
          )}
          <span className="text-xs text-blue-400">{relativeTime(aiResult.createdAt)}</span>
        </div>
      </div>

      {/* Summary */}
      {aiResult.summary && (
        <MarkdownContent text={aiResult.summary} className="mb-4 font-medium text-slate-800 dark:text-slate-200" />
      )}

      {/* Suggestions */}
      {aiResult.suggestions && aiResult.suggestions.length > 0 && (
        <div className="mb-4">
          <h3 className="mb-2 flex items-center gap-1.5 text-xs font-semibold uppercase text-blue-600 dark:text-blue-400">
            <Activity className="h-3 w-3" />
            Recommended Actions
          </h3>
          <ul className="space-y-1.5">
            {aiResult.suggestions.map((s, i) => (
              <li key={i} className="flex items-start gap-2 text-sm text-slate-600 dark:text-slate-400">
                <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-blue-400" />
                {s}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Expandable full analysis */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-1 text-xs font-medium text-blue-600 transition-colors hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
      >
        <ChevronDown className={cn('h-3.5 w-3.5 transition-transform', expanded && 'rotate-180')} />
        {expanded ? 'Hide' : 'Show'} full analysis
      </button>
      {expanded && (
        <div className="mt-3 max-h-80 overflow-auto rounded-lg bg-slate-900/5 p-4 dark:bg-slate-950/50">
          <MarkdownContent text={aiResult.analysis} className="text-xs text-slate-600 dark:text-slate-400" />
        </div>
      )}
    </div>
  )
}

function TimelineEntry({ label, time, detail, color }: { label: string; time: string; detail?: string; color: string }) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className={cn('h-2.5 w-2.5 rounded-full', color)} />
        <div className="w-px flex-1 bg-slate-200 dark:bg-slate-700" />
      </div>
      <div className="pb-4">
        <p className="text-sm font-medium text-slate-900 dark:text-slate-100">{label}</p>
        <p className="text-xs text-slate-400">{formatDate(time, 'MMM d, yyyy HH:mm:ss')}</p>
        {detail && <p className="mt-0.5 text-xs text-slate-500">{detail}</p>}
      </div>
    </div>
  )
}
