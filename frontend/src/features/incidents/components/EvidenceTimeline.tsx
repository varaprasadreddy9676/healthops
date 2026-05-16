import { useQuery } from '@tanstack/react-query'
import { Clock, Loader2 } from 'lucide-react'
import { cn, formatDate } from '@/shared/lib/utils'
import { evidenceApi, type IncidentTimelineEvent } from '@/features/incidents/api/evidence'

const eventTypeColors: Record<string, string> = {
  incident_created: 'bg-red-500',
  incident_acknowledged: 'bg-amber-500',
  incident_resolved: 'bg-emerald-500',
  incident_escalated: 'bg-orange-500',
  check_failed: 'bg-red-400',
  check_recovered: 'bg-emerald-400',
  mysql_anomaly: 'bg-purple-500',
  server_anomaly: 'bg-blue-500',
  audit_action: 'bg-slate-400',
  ai_brief_generated: 'bg-indigo-500',
}

const eventTypeLabels: Record<string, string> = {
  incident_created: 'Incident Created',
  incident_acknowledged: 'Acknowledged',
  incident_resolved: 'Resolved',
  incident_escalated: 'Escalated',
  check_failed: 'Check Failed',
  check_recovered: 'Check Recovered',
  mysql_anomaly: 'MySQL Anomaly',
  server_anomaly: 'Server Anomaly',
  audit_action: 'Audit Action',
  ai_brief_generated: 'AI Brief Generated',
}

function EventEntry({ event, isLast }: { event: IncidentTimelineEvent; isLast: boolean }) {
  const dotColor = eventTypeColors[event.type] || 'bg-slate-400'
  const label = eventTypeLabels[event.type] || event.type.replace(/_/g, ' ')

  return (
    <div className="flex items-start gap-2.5">
      <div className="relative flex flex-col items-center">
        <div className={cn('h-2 w-2 rounded-full', dotColor)} />
        {!isLast && (
          <div className="absolute top-2.5 h-full w-px bg-slate-200 dark:bg-slate-700" />
        )}
      </div>
      <div className="min-w-0 pb-4">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[10px] font-medium text-slate-400">
            {formatDate(event.timestamp, 'MMM d, HH:mm:ss')}
          </span>
          <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium capitalize text-slate-500 dark:bg-slate-800">
            {label}
          </span>
          {event.source && (
            <span className="text-[10px] text-slate-400">via {event.source}</span>
          )}
        </div>
        <p className="mt-0.5 text-xs text-slate-600 dark:text-slate-400">{event.message}</p>
        {event.attributes && Object.keys(event.attributes).length > 0 && (
          <div className="mt-1 flex flex-wrap gap-1.5">
            {Object.entries(event.attributes).map(([k, v]) => (
              <span key={k} className="rounded bg-slate-50 px-1.5 py-0.5 text-[10px] text-slate-500 dark:bg-slate-800/50">
                {k}: {v}
              </span>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export function EvidenceTimeline({ incidentId }: { incidentId: string }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['evidence', 'timeline', incidentId],
    queryFn: () => evidenceApi.getTimeline(incidentId),
    enabled: !!incidentId,
    retry: false,
  })

  const events = data?.events ?? []

  // Don't render the panel if loading failed or no events
  if (error || (!isLoading && events.length === 0)) return null

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-slate-600 dark:text-slate-400" />
          <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">
            Evidence Timeline
          </h2>
        </div>
        {events.length > 0 && (
          <span className="text-[10px] font-medium uppercase text-slate-400">
            {events.length} event{events.length !== 1 ? 's' : ''}
          </span>
        )}
      </div>

      {isLoading && (
        <div className="flex items-center gap-2 text-sm text-slate-500">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading timeline...
        </div>
      )}

      {!isLoading && events.length > 0 && (
        <div className="space-y-0">
          {events.map((event, i) => (
            <EventEntry key={event.id} event={event} isLast={i === events.length - 1} />
          ))}
        </div>
      )}
    </div>
  )
}
