import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Sparkles, ChevronDown, Target, Clock, AlertTriangle, CheckCircle2, Info, Loader2 } from 'lucide-react'
import { cn, relativeTime, formatDate } from '@/shared/lib/utils'
import { evidenceApi, type IncidentBrief } from '@/features/incidents/api/evidence'

function ConfidenceBadge({ confidence }: { confidence: IncidentBrief['confidence'] }) {
  const pct = Math.round(confidence.score * 100)
  const color =
    confidence.band === 'high'
      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400'
      : confidence.band === 'medium'
        ? 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400'
        : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'

  return (
    <span className={cn('inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase', color)}>
      {confidence.band} ({pct}%)
    </span>
  )
}

function ConfidenceBreakdownPanel({ breakdown }: { breakdown: IncidentBrief['confidence']['breakdown'] }) {
  const factors = [
    { label: 'Deployment correlation', active: breakdown.hasDeploymentCorrelation },
    { label: 'Metric anomaly', active: breakdown.hasMetricAnomaly },
    { label: 'Log fingerprint spike', active: breakdown.hasLogFingerprintSpike },
    { label: 'Similar past incident', active: breakdown.hasSimilarPastIncident },
  ]

  return (
    <div className="mt-2 rounded-lg bg-slate-50 p-3 dark:bg-slate-800/50">
      <span className="text-[10px] font-semibold uppercase text-slate-400">Confidence Factors</span>
      <div className="mt-1.5 grid grid-cols-2 gap-1.5">
        {factors.map((f) => (
          <div key={f.label} className="flex items-center gap-1.5 text-xs">
            {f.active ? (
              <CheckCircle2 className="h-3 w-3 text-emerald-500" />
            ) : (
              <div className="h-3 w-3 rounded-full border border-slate-300 dark:border-slate-600" />
            )}
            <span className={f.active ? 'text-slate-700 dark:text-slate-300' : 'text-slate-400 dark:text-slate-500'}>
              {f.label}
            </span>
          </div>
        ))}
      </div>
      <div className="mt-2 flex items-center gap-2 text-[10px] text-slate-400">
        <span>{breakdown.evidenceCount} evidence signals</span>
        <span>{breakdown.availableCategories}/{breakdown.totalPossibleCategories} categories</span>
      </div>
    </div>
  )
}

function BriefContent({ brief }: { brief: IncidentBrief }) {
  const [showBreakdown, setShowBreakdown] = useState(false)
  const [showRaw, setShowRaw] = useState(false)

  return (
    <div className="space-y-4">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Sparkles className="h-4 w-4 text-indigo-600 dark:text-indigo-400" />
          <h2 className="text-sm font-semibold text-indigo-900 dark:text-indigo-300">AI Incident Brief</h2>
        </div>
        <div className="flex items-center gap-2">
          <ConfidenceBadge confidence={brief.confidence} />
          <span className="text-xs text-indigo-400">{relativeTime(brief.generatedAt)}</span>
        </div>
      </div>

      {/* Likely cause */}
      <div>
        <div className="mb-1 flex items-center gap-1 text-[10px] font-semibold uppercase text-indigo-600 dark:text-indigo-400">
          <Target className="h-3 w-3" />
          Likely Cause
        </div>
        <p className="text-sm font-medium leading-relaxed text-slate-800 dark:text-slate-200">
          {brief.likelyCause}
        </p>
      </div>

      {/* Impact */}
      {brief.impactSummary && (
        <div>
          <div className="mb-1 flex items-center gap-1 text-[10px] font-semibold uppercase text-amber-600 dark:text-amber-400">
            <AlertTriangle className="h-3 w-3" />
            Impact
          </div>
          <p className="text-sm text-slate-600 dark:text-slate-400">{brief.impactSummary}</p>
        </div>
      )}

      {/* Next actions */}
      {brief.nextActions?.length > 0 && (
        <div>
          <h3 className="mb-1.5 flex items-center gap-1 text-[10px] font-semibold uppercase text-indigo-600 dark:text-indigo-400">
            <CheckCircle2 className="h-3 w-3" />
            Recommended Actions
          </h3>
          <ol className="space-y-1">
            {brief.nextActions.map((action, i) => (
              <li key={i} className="flex items-start gap-2 text-sm text-slate-600 dark:text-slate-400">
                <span className="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-indigo-100 text-[10px] font-bold text-indigo-700 dark:bg-indigo-900/50 dark:text-indigo-300">
                  {i + 1}
                </span>
                {action}
              </li>
            ))}
          </ol>
        </div>
      )}

      {/* Brief timeline */}
      {brief.timeline && brief.timeline.length > 0 && (
        <div>
          <h3 className="mb-1.5 flex items-center gap-1 text-[10px] font-semibold uppercase text-slate-500">
            <Clock className="h-3 w-3" />
            Event Sequence
          </h3>
          <div className="space-y-1.5">
            {brief.timeline.map((entry, i) => (
              <div key={i} className="flex items-start gap-2 text-xs">
                <span className="mt-0.5 shrink-0 font-mono text-slate-400">{entry.time}</span>
                <span className="text-slate-600 dark:text-slate-400">{entry.description}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Evidence citations */}
      {brief.evidenceSummary?.length > 0 && (
        <div>
          <h3 className="mb-1.5 flex items-center gap-1 text-[10px] font-semibold uppercase text-slate-500">
            <Info className="h-3 w-3" />
            Evidence ({brief.evidenceSummary.length})
          </h3>
          <div className="space-y-1">
            {brief.evidenceSummary.map((cite, i) => (
              <div key={i} className="flex items-start gap-2 text-xs">
                <span className="mt-0.5 shrink-0 rounded bg-slate-100 px-1.5 py-0.5 font-medium text-slate-500 dark:bg-slate-800">
                  {cite.category}
                </span>
                <span className="text-slate-600 dark:text-slate-400">{cite.description}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Footer controls */}
      <div className="flex flex-wrap gap-3 border-t border-indigo-100 pt-3 dark:border-indigo-900/50">
        <button
          onClick={() => setShowBreakdown(!showBreakdown)}
          className="flex items-center gap-1 text-xs font-medium text-indigo-600 transition-colors hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300"
        >
          <ChevronDown className={cn('h-3.5 w-3.5 transition-transform', showBreakdown && 'rotate-180')} />
          Confidence breakdown
        </button>
        {brief.rawAiResponse && (
          <button
            onClick={() => setShowRaw(!showRaw)}
            className="flex items-center gap-1 text-xs font-medium text-indigo-600 transition-colors hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300"
          >
            <ChevronDown className={cn('h-3.5 w-3.5 transition-transform', showRaw && 'rotate-180')} />
            Raw AI response
          </button>
        )}
        {brief.metadata && (
          <span className="ml-auto text-[10px] text-slate-400">
            {brief.metadata.evidenceCount} signals in {brief.metadata.durationMs}ms
            {brief.metadata.wasCapped && ' (capped)'}
          </span>
        )}
      </div>

      {showBreakdown && <ConfidenceBreakdownPanel breakdown={brief.confidence.breakdown} />}

      {showRaw && (
        <pre className="mt-2 max-h-60 overflow-auto rounded-lg bg-slate-900/5 p-3 font-mono text-xs leading-relaxed text-slate-600 dark:bg-slate-950/50 dark:text-slate-400">
          {brief.rawAiResponse}
        </pre>
      )}
    </div>
  )
}

export function EvidenceBriefCard({ incidentId }: { incidentId: string }) {
  const queryClient = useQueryClient()

  const { data: brief, isLoading, error } = useQuery({
    queryKey: ['evidence', 'brief', incidentId],
    queryFn: () => evidenceApi.getBrief(incidentId),
    enabled: !!incidentId,
    retry: false,
  })

  const generateMutation = useMutation({
    mutationFn: () => evidenceApi.generateBrief(incidentId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['evidence', 'brief', incidentId] })
    },
  })

  const hasBrief = !!brief && !error

  return (
    <div className="rounded-xl border border-indigo-200 bg-gradient-to-br from-indigo-50/80 to-violet-50/40 p-5 dark:border-indigo-900 dark:from-indigo-950/30 dark:to-violet-950/20">
      {isLoading && (
        <div className="flex items-center gap-2 text-sm text-slate-500">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading brief...
        </div>
      )}

      {hasBrief && <BriefContent brief={brief} />}

      {!hasBrief && !isLoading && (
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Sparkles className="h-4 w-4 text-indigo-600 dark:text-indigo-400" />
            <h2 className="text-sm font-semibold text-indigo-900 dark:text-indigo-300">AI Incident Brief</h2>
          </div>
          <button
            onClick={() => generateMutation.mutate()}
            disabled={generateMutation.isPending}
            className="inline-flex items-center gap-1.5 rounded-lg border border-indigo-200 bg-white/80 px-3 py-1.5 text-xs font-medium text-indigo-700 transition-colors hover:bg-indigo-50 disabled:opacity-50 dark:border-indigo-800 dark:bg-indigo-950/50 dark:text-indigo-400"
          >
            {generateMutation.isPending ? (
              <>
                <Loader2 className="h-3 w-3 animate-spin" />
                Generating...
              </>
            ) : (
              <>
                <Sparkles className="h-3 w-3" />
                Generate Brief
              </>
            )}
          </button>
        </div>
      )}

      {generateMutation.isError && (
        <p className="mt-2 text-xs text-red-600 dark:text-red-400">
          {(generateMutation.error as Error).message || 'Failed to generate brief'}
        </p>
      )}

      {/* Re-generate button when brief exists */}
      {hasBrief && (
        <div className="mt-3 flex justify-end">
          <button
            onClick={() => generateMutation.mutate()}
            disabled={generateMutation.isPending}
            className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium text-indigo-500 transition-colors hover:bg-indigo-100 hover:text-indigo-700 dark:hover:bg-indigo-900/50 dark:hover:text-indigo-300"
          >
            {generateMutation.isPending ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              <Sparkles className="h-3 w-3" />
            )}
            Regenerate
          </button>
        </div>
      )}
    </div>
  )
}
