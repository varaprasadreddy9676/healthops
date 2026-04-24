import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { useState } from 'react'
import { ArrowLeft, Server, Tag, Pencil, X, Save, Bell } from 'lucide-react'
import { checksApi } from "@/features/checks/api/checks"
import { analyticsApi } from "@/features/analytics/api/analytics"
import { StatusBadge } from "@/shared/components/StatusBadge"
import { MetricCard } from "@/shared/components/MetricCard"
import { LoadingState } from "@/shared/components/LoadingState"
import { ErrorState } from "@/shared/components/ErrorState"
import { ResponseTimeChart } from "@/shared/charts/ResponseTimeChart"
import { formatDuration, formatUptime, relativeTime, checkTypeLabel } from "@/shared/lib/utils"
import { REFETCH_INTERVAL } from "@/shared/lib/constants"
import type { CheckConfig } from "@/shared/types"

interface NotificationChannel {
  id: string
  name: string
  type: string
  enabled: boolean
}

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('healthops_token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function fetchChannels(): Promise<NotificationChannel[]> {
  const res = await fetch('/api/v1/notification-channels', { headers: authHeaders() })
  if (!res.ok) return []
  const body = await res.json()
  return body.data || []
}

export default function CheckDetail() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [form, setForm] = useState<Partial<CheckConfig>>({})

  const { data: detail, isLoading, error, refetch } = useQuery({
    queryKey: ['checks', id],
    queryFn: () => checksApi.get(id!),
    enabled: !!id,
    refetchInterval: REFETCH_INTERVAL,
  })

  const { data: rtData } = useQuery({
    queryKey: ['analytics', 'response-times', id, '24h'],
    queryFn: () => analyticsApi.responseTimes({ checkId: id, period: '24h', interval: '1h' }),
    enabled: !!id,
  })

  const { data: channels = [] } = useQuery({
    queryKey: ['notification-channels-list'],
    queryFn: fetchChannels,
    enabled: editing,
  })

  const updateMutation = useMutation({
    mutationFn: (data: Partial<CheckConfig>) => checksApi.update(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['checks', id] })
      queryClient.invalidateQueries({ queryKey: ['checks'] })
      setEditing(false)
    },
  })

  const openEdit = () => {
    if (!detail) return
    const c = detail.config
    setForm({
      name: c.name,
      type: c.type,
      target: c.target,
      server: c.server,
      application: c.application,
      host: c.host,
      port: c.port,
      command: c.command,
      path: c.path,
      expectedStatus: c.expectedStatus,
      expectedContains: c.expectedContains,
      timeoutSeconds: c.timeoutSeconds,
      warningThresholdMs: c.warningThresholdMs,
      freshnessSeconds: c.freshnessSeconds,
      intervalSeconds: c.intervalSeconds,
      retryCount: c.retryCount,
      retryDelaySeconds: c.retryDelaySeconds,
      cooldownSeconds: c.cooldownSeconds,
      enabled: c.enabled,
      tags: c.tags || [],
      notificationChannelIds: c.notificationChannelIds || [],
    })
    setEditing(true)
  }

  const handleSave = () => {
    updateMutation.mutate(form)
  }

  const toggleChannel = (channelId: string) => {
    setForm(prev => {
      const ids = prev.notificationChannelIds || []
      return {
        ...prev,
        notificationChannelIds: ids.includes(channelId)
          ? ids.filter(id => id !== channelId)
          : [...ids, channelId],
      }
    })
  }

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState message={error.message} retry={() => refetch()} />
  if (!detail) return null

  const { config, latestResult, uptime24h, uptime7d, avgDurationMs, recentResults, openIncidents } = detail

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Breadcrumb */}
      <div className="flex items-center gap-3">
        <Link to="/checks" className="rounded-md p-1 text-slate-400 transition-colors hover:text-slate-600 dark:hover:text-slate-300">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div>
          <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">{config.name}</h1>
          <div className="mt-0.5 flex items-center gap-2 text-xs text-slate-500">
            <span className="rounded bg-slate-100 px-1.5 py-0.5 font-medium dark:bg-slate-800">{checkTypeLabel(config.type)}</span>
            {config.server && <span className="flex items-center gap-1"><Server className="h-3 w-3" /> {config.server}</span>}
            {config.target && <span className="truncate max-w-[200px]">{config.target}</span>}
          </div>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <button
            onClick={openEdit}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
          >
            <Pencil className="h-3.5 w-3.5" /> Edit
          </button>
          {latestResult && <StatusBadge status={latestResult.status} size="md" />}
        </div>
      </div>

      {/* Metric cards */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <MetricCard label="Uptime (24h)" value={formatUptime(uptime24h)} />
        <MetricCard label="Uptime (7d)" value={formatUptime(uptime7d)} />
        <MetricCard label="Avg Response" value={formatDuration(avgDurationMs)} />
        <MetricCard
          label="Last Check"
          value={latestResult ? formatDuration(latestResult.durationMs) : '—'}
          subValue={latestResult ? relativeTime(latestResult.finishedAt) : undefined}
        />
      </div>

      {/* Response time chart */}
      {rtData && rtData.length > 0 && (
        <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
          <h2 className="mb-4 text-sm font-semibold text-slate-900 dark:text-slate-100">Response Time (24h)</h2>
          <ResponseTimeChart data={rtData} showPercentiles />
        </div>
      )}

      {/* Open incidents */}
      {openIncidents && openIncidents.length > 0 && (
        <div className="rounded-xl border border-red-200 bg-red-50/50 p-5 dark:border-red-900 dark:bg-red-950/20">
          <h2 className="mb-3 text-sm font-semibold text-red-700 dark:text-red-400">
            Open Incidents ({openIncidents.length})
          </h2>
          <div className="space-y-2">
            {openIncidents.map(inc => (
              <Link
                key={inc.id}
                to={`/incidents/${inc.id}`}
                className="flex items-center gap-3 rounded-lg bg-white p-3 transition-colors hover:bg-red-50 dark:bg-slate-900 dark:hover:bg-slate-800"
              >
                <StatusBadge status={inc.severity} label={false} />
                <span className="text-sm text-slate-700 dark:text-slate-300">{inc.message}</span>
                <span className="ml-auto text-xs text-slate-400">{relativeTime(inc.startedAt)}</span>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Recent results table */}
      <div className="rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
        <div className="border-b border-slate-100 px-5 py-3.5 dark:border-slate-800">
          <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">Recent Results</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-100 bg-slate-50/50 dark:border-slate-800 dark:bg-slate-800/30">
                <th className="px-4 py-2.5 text-left text-xs font-semibold uppercase text-slate-500">Status</th>
                <th className="px-4 py-2.5 text-left text-xs font-semibold uppercase text-slate-500">Response</th>
                <th className="px-4 py-2.5 text-left text-xs font-semibold uppercase text-slate-500">Message</th>
                <th className="px-4 py-2.5 text-left text-xs font-semibold uppercase text-slate-500">Time</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {recentResults.slice(0, 20).map(r => (
                <tr key={r.id}>
                  <td className="px-4 py-2.5"><StatusBadge status={r.status} /></td>
                  <td className="px-4 py-2.5 font-mono text-xs">{formatDuration(r.durationMs)}</td>
                  <td className="max-w-xs truncate px-4 py-2.5 text-slate-500 dark:text-slate-400">{r.message || '—'}</td>
                  <td className="px-4 py-2.5 text-xs text-slate-400">{relativeTime(r.finishedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Config details */}
      <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
        <h2 className="mb-3 text-sm font-semibold text-slate-900 dark:text-slate-100">Configuration</h2>
        <dl className="grid gap-x-8 gap-y-2 sm:grid-cols-2 lg:grid-cols-3">
          {([
            ['ID', config.id],
            ['Type', checkTypeLabel(config.type)],
            ['Target', config.target],
            ['Server', config.server],
            ['Application', config.application],
            ['Timeout', config.timeoutSeconds ? `${config.timeoutSeconds}s` : 'Default'],
            ['Warning Threshold', config.warningThresholdMs ? `${config.warningThresholdMs}ms` : 'None'],
            ['Interval', config.intervalSeconds ? `${config.intervalSeconds}s` : 'Default'],
            ['Enabled', config.enabled !== false ? 'Yes' : 'No'],
          ] as const).filter(([, v]) => v).map(([label, value]) => (
            <div key={label} className="flex gap-2 text-sm">
              <dt className="font-medium text-slate-500 dark:text-slate-400">{label}:</dt>
              <dd className="text-slate-700 dark:text-slate-300">{value}</dd>
            </div>
          ))}
        </dl>
        {config.tags && config.tags.length > 0 && (
          <div className="mt-3 flex items-center gap-2">
            <Tag className="h-3.5 w-3.5 text-slate-400" />
            {config.tags.map(tag => (
              <span key={tag} className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-400">{tag}</span>
            ))}
          </div>
        )}
      </div>

      {/* Edit Check Modal */}
      {editing && (
        <div className="fixed inset-0 z-[90] flex items-center justify-center">
          <div className="fixed inset-0 bg-slate-900/50" onClick={() => setEditing(false)} />
          <div className="relative z-10 w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-700 dark:bg-slate-900 animate-slide-up">
            <div className="flex items-center justify-between mb-5">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Edit Check</h3>
              <button onClick={() => setEditing(false)} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300">
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="space-y-4">
              {/* Basic fields */}
              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Name</label>
                  <input
                    value={form.name || ''}
                    onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Type</label>
                  <input value={checkTypeLabel(form.type || 'api')} disabled className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800/50 dark:text-slate-400 cursor-not-allowed" />
                </div>
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Target / URL</label>
                  <input
                    value={form.target || ''}
                    onChange={e => setForm(f => ({ ...f, target: e.target.value }))}
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Server</label>
                  <input
                    value={form.server || ''}
                    onChange={e => setForm(f => ({ ...f, server: e.target.value }))}
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                  />
                </div>
              </div>

              <div className="grid gap-4 sm:grid-cols-3">
                <div>
                  <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Timeout (s)</label>
                  <input
                    type="number"
                    value={form.timeoutSeconds ?? ''}
                    onChange={e => setForm(f => ({ ...f, timeoutSeconds: e.target.value ? Number(e.target.value) : undefined }))}
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Warning (ms)</label>
                  <input
                    type="number"
                    value={form.warningThresholdMs ?? ''}
                    onChange={e => setForm(f => ({ ...f, warningThresholdMs: e.target.value ? Number(e.target.value) : undefined }))}
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Interval (s)</label>
                  <input
                    type="number"
                    value={form.intervalSeconds ?? ''}
                    onChange={e => setForm(f => ({ ...f, intervalSeconds: e.target.value ? Number(e.target.value) : undefined }))}
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                  />
                </div>
              </div>

              <div className="flex items-center gap-3">
                <label className="relative inline-flex cursor-pointer items-center">
                  <input
                    type="checkbox"
                    checked={form.enabled !== false}
                    onChange={e => setForm(f => ({ ...f, enabled: e.target.checked }))}
                    className="peer sr-only"
                  />
                  <div className="h-5 w-9 rounded-full bg-slate-200 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:bg-white after:transition-all peer-checked:bg-emerald-500 peer-checked:after:translate-x-full dark:bg-slate-700" />
                </label>
                <span className="text-sm text-slate-700 dark:text-slate-300">Enabled</span>
              </div>

              {/* Notification Channels section */}
              <div className="border-t border-slate-200 pt-4 dark:border-slate-700">
                <div className="flex items-center gap-2 mb-3">
                  <Bell className="h-4 w-4 text-slate-500" />
                  <h4 className="text-sm font-semibold text-slate-900 dark:text-slate-100">Notification Channels</h4>
                </div>
                <p className="text-xs text-slate-500 mb-3">
                  Select which channels should be notified when this check triggers an incident. Channels with their own matching filters will also apply.
                </p>
                {channels.length === 0 ? (
                  <p className="text-xs text-slate-400 italic">No notification channels configured. Create channels in the Notifications page first.</p>
                ) : (
                  <div className="grid gap-2 sm:grid-cols-2">
                    {channels.map(ch => (
                      <label
                        key={ch.id}
                        className={`flex items-center gap-3 rounded-lg border p-3 text-sm cursor-pointer transition-colors ${
                          (form.notificationChannelIds || []).includes(ch.id)
                            ? 'border-blue-300 bg-blue-50/50 dark:border-blue-700 dark:bg-blue-950/30'
                            : 'border-slate-200 bg-white hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:hover:bg-slate-750'
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={(form.notificationChannelIds || []).includes(ch.id)}
                          onChange={() => toggleChannel(ch.id)}
                          className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500 dark:border-slate-600"
                        />
                        <div className="flex-1 min-w-0">
                          <span className="font-medium text-slate-700 dark:text-slate-300">{ch.name}</span>
                          <span className="ml-2 rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-700 dark:text-slate-400">
                            {ch.type}
                          </span>
                        </div>
                        {!ch.enabled && (
                          <span className="text-xs text-amber-600 dark:text-amber-400">Disabled</span>
                        )}
                      </label>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {/* Actions */}
            <div className="mt-6 flex items-center justify-end gap-3 border-t border-slate-200 pt-4 dark:border-slate-700">
              {updateMutation.isError && (
                <span className="mr-auto text-sm text-red-600">Failed to save changes</span>
              )}
              <button
                onClick={() => setEditing(false)}
                className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={updateMutation.isPending}
                className="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                <Save className="h-3.5 w-3.5" />
                {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
