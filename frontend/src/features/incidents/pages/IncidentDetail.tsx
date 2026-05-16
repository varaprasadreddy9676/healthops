import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, CheckCircle, Eye, Brain, ChevronDown, ShieldAlert, Activity } from 'lucide-react'
import { useState } from 'react'
import { incidentsApi } from "@/features/incidents/api/incidents"
import { aiApi } from "@/features/ai/api/ai"
import { LoadingState } from "@/shared/components/LoadingState"
import { ErrorState } from "@/shared/components/ErrorState"
import { cn, relativeTime, formatDate, incidentStatusLabel, severityColor } from "@/shared/lib/utils"

export default function IncidentDetail() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()

  const { data: incident, isLoading, error, refetch } = useQuery({
    queryKey: ['incidents', id],
    queryFn: () => incidentsApi.get(id!),
    enabled: !!id,
  })

  const { data: snapshots } = useQuery({
    queryKey: ['incidents', id, 'snapshots'],
    queryFn: () => incidentsApi.snapshots(id!),
    enabled: !!id,
  })

  const { data: aiResults } = useQuery({
    queryKey: ['ai', 'results', id],
    queryFn: () => aiApi.results(id!),
    enabled: !!id,
    retry: false,
  })

  const { data: aiConfig } = useQuery({
    queryKey: ['ai', 'config'],
    queryFn: aiApi.config,
    retry: false,
    staleTime: 60_000,
  })

  const aiEnabled = aiConfig?.enabled && (aiConfig?.providers?.length ?? 0) > 0

  const ackMutation = useMutation({
    mutationFn: () => incidentsApi.acknowledge(id!),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['incidents'] }) },
  })

  const resolveMutation = useMutation({
    mutationFn: () => incidentsApi.resolve(id!),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['incidents'] }) },
  })

  const analyzeMutation = useMutation({
    mutationFn: () => aiApi.analyze(id!),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['ai', 'results', id] }) },
  })

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState message={error.message} retry={() => refetch()} />
  if (!incident) return null
  const aiResult = aiResults?.[0]

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
          <p className="mt-1 text-sm text-slate-500">{incident.message}</p>
        </div>
      </div>

      {/* Actions */}
      {incident.status !== 'resolved' && (
        <div className="flex gap-2">
          {incident.status === 'open' && (
            <button
              onClick={() => ackMutation.mutate()}
              disabled={ackMutation.isPending}
              className="inline-flex items-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2 text-sm font-medium text-amber-700 transition-colors hover:bg-amber-100 disabled:opacity-50 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400"
            >
              <Eye className="h-4 w-4" />
              Acknowledge
            </button>
          )}
          <button
            onClick={() => resolveMutation.mutate()}
            disabled={resolveMutation.isPending}
            className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm font-medium text-emerald-700 transition-colors hover:bg-emerald-100 disabled:opacity-50 dark:border-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-400"
          >
            <CheckCircle className="h-4 w-4" />
            Resolve
          </button>
          {aiEnabled && (
            <button
              onClick={() => analyzeMutation.mutate()}
              disabled={analyzeMutation.isPending}
              className="inline-flex items-center gap-1.5 rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-sm font-medium text-blue-700 transition-colors hover:bg-blue-100 disabled:opacity-50 dark:border-blue-800 dark:bg-blue-950/30 dark:text-blue-400"
            >
              <Brain className="h-4 w-4" />
              {analyzeMutation.isPending ? 'Analyzing...' : 'AI Analysis'}
            </button>
          )}
        </div>
      )}

      {/* Timeline */}
      <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
        <h2 className="mb-4 text-sm font-semibold text-slate-900 dark:text-slate-100">Timeline</h2>
        <div className="space-y-4">
          <TimelineEntry
            label="Started"
            time={incident.startedAt}
            detail={incident.message}
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

      {/* AI Analysis */}
      {aiResult && <AIAnalysisCard aiResult={aiResult} />}

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

      {/* Metadata */}
      {incident.metadata && Object.keys(incident.metadata).length > 0 && (
        <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
          <h2 className="mb-3 text-sm font-semibold text-slate-900 dark:text-slate-100">Metadata</h2>
          <dl className="grid gap-2 sm:grid-cols-2">
            {Object.entries(incident.metadata).map(([k, v]) => (
              <div key={k} className="text-sm">
                <dt className="font-medium text-slate-500">{k}</dt>
                <dd className="text-slate-700 dark:text-slate-300">{v}</dd>
              </div>
            ))}
          </dl>
        </div>
      )}
    </div>
  )
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
    <div className="rounded-xl border border-blue-200 bg-gradient-to-br from-blue-50/80 to-indigo-50/40 p-5 dark:border-blue-900 dark:from-blue-950/30 dark:to-indigo-950/20">
      {/* Header */}
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Brain className="h-4 w-4 text-blue-600 dark:text-blue-400" />
          <h2 className="text-sm font-semibold text-blue-900 dark:text-blue-300">AI Analysis</h2>
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
        <p className="mb-4 text-sm font-medium leading-relaxed text-slate-800 dark:text-slate-200">
          {aiResult.summary}
        </p>
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
        <pre className="mt-3 max-h-80 overflow-auto rounded-lg bg-slate-900/5 p-4 font-mono text-xs leading-relaxed text-slate-600 dark:bg-slate-950/50 dark:text-slate-400">
          {aiResult.analysis}
        </pre>
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
