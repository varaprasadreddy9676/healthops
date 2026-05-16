import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Bell, Plus, Trash2, Pencil, ToggleLeft, ToggleRight, Send, Mail, Globe, MessageSquare, Hash, AlertTriangle, Filter, ChevronDown, ChevronRight, CheckCircle, XCircle, Clock } from 'lucide-react'
import { useAuth } from "@/shared/hooks/useAuth"
import { LoadingState } from "@/shared/components/LoadingState"
import { ErrorState } from "@/shared/components/ErrorState"
import { EmptyState } from "@/shared/components/EmptyState"
import { useToast } from "@/shared/components/Toast"
import { useConfirm } from "@/shared/components/ConfirmDialog"
import { checksApi } from "@/features/checks/api/checks"

interface NotificationChannel {
  id: string
  name: string
  type: string
  enabled: boolean
  webhookUrl?: string
  email?: string
  smtpHost?: string
  smtpPort?: number
  smtpUser?: string
  fromEmail?: string
  botToken?: string
  chatId?: string
  routingKey?: string
  severities?: string[]
  checkIds?: string[]
  checkTypes?: string[]
  servers?: string[]
  tags?: string[]
  cooldownMinutes?: number
  notifyOnResolve?: boolean
  headers?: Record<string, string>
  bodyTemplate?: string
  createdAt?: string
  updatedAt?: string
}

interface NotificationLog {
  notificationId: string
  incidentId: string
  channel: string
  payloadJson: string
  status: 'pending' | 'sent' | 'failed'
  lastError?: string
  retryCount: number
  createdAt: string
  sentAt?: string
  requestUrl?: string
  responseStatus?: number
  responseBody?: string
}

const CHANNEL_TYPES = [
  { value: 'email', label: 'Email', icon: Mail },
  { value: 'webhook', label: 'Webhook', icon: Globe },
  { value: 'slack', label: 'Slack', icon: Hash },
  { value: 'discord', label: 'Discord', icon: MessageSquare },
  { value: 'telegram', label: 'Telegram', icon: Send },
  { value: 'pagerduty', label: 'PagerDuty', icon: AlertTriangle },
]

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('healthops_token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function fetchChannels(): Promise<NotificationChannel[]> {
  const res = await fetch('/api/v1/notification-channels', { headers: authHeaders() })
  if (!res.ok) throw new Error('Failed to load channels')
  const body = await res.json()
  return body.data || []
}

async function fetchNotificationLogs(status: string, channel: string, limit: number): Promise<NotificationLog[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (status) params.set('status', status)
  if (channel) params.set('channel', channel)
  const res = await fetch(`/api/v1/notification-logs?${params}`, { headers: authHeaders() })
  if (!res.ok) throw new Error('Failed to load notification logs')
  const body = await res.json()
  return body.data || []
}

const emptyForm: Partial<NotificationChannel> = {
  name: '', type: 'webhook', enabled: true, webhookUrl: '', email: '',
  smtpHost: '', smtpPort: 587, smtpUser: '', fromEmail: '',
  botToken: '', chatId: '', routingKey: '', cooldownMinutes: 5,
  severities: [], checkIds: [], checkTypes: [], servers: [], tags: [],
  notifyOnResolve: true,
}

function validateChannelForm(data: Partial<NotificationChannel>) {
  if (!data.name?.trim()) return 'Channel name is required'

  switch (data.type) {
    case 'webhook':
    case 'slack':
    case 'discord':
      return data.webhookUrl?.trim() ? null : 'Webhook URL is required'
    case 'email':
      if (!data.email?.trim()) return 'Recipient email is required'
      if (!data.fromEmail?.trim()) return 'From email is required'
      if (!data.smtpHost?.trim()) return 'SMTP host is required'
      if (!data.smtpPort || data.smtpPort < 1 || data.smtpPort > 65535) return 'SMTP port must be between 1 and 65535'
      return null
    case 'telegram':
      if (!data.botToken?.trim()) return 'Telegram bot token is required'
      if (!data.chatId?.trim()) return 'Telegram chat ID is required'
      return null
    case 'pagerduty':
      return data.routingKey?.trim() ? null : 'PagerDuty routing key is required'
    default:
      return null
  }
}

function StatusBadge({ status }: { status: string }) {
  if (status === 'sent') {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950/50 dark:text-green-400">
        <CheckCircle className="h-3 w-3" /> sent
      </span>
    )
  }
  if (status === 'failed') {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-950/50 dark:text-red-400">
        <XCircle className="h-3 w-3" /> failed
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-950/50 dark:text-amber-400">
      <Clock className="h-3 w-3" /> pending
    </span>
  )
}

function prettyJson(s: string) {
  try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s }
}

function buildCurl(log: NotificationLog): string {
  if (!log.requestUrl) return '(URL not recorded — will appear for future alerts)'
  let cmd = `curl -X POST '${log.requestUrl}' \\\n  -H 'Content-Type: application/json'`
  const payload = prettyJson(log.payloadJson)
  cmd += ` \\\n  -d '${payload}'`
  return cmd
}

function LogRow({ log }: { log: NotificationLog }) {
  const [open, setOpen] = useState(false)
  const fmtTime = (s?: string) => {
    if (!s) return '—'
    try { return new Date(s).toLocaleString() } catch { return s }
  }

  return (
    <>
      <tr
        className="cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-900/30"
        onClick={() => setOpen(o => !o)}
      >
        <td className="px-4 py-3">
          <div className="flex items-center gap-2">
            {open ? <ChevronDown className="h-3.5 w-3.5 text-slate-400" /> : <ChevronRight className="h-3.5 w-3.5 text-slate-400" />}
            <StatusBadge status={log.status} />
          </div>
        </td>
        <td className="px-4 py-3 font-mono text-xs text-slate-700 dark:text-slate-300">{log.channel}</td>
        <td className="px-4 py-3 font-mono text-xs text-slate-500 max-w-[140px] truncate" title={log.incidentId}>
          {log.incidentId.length > 18 ? log.incidentId.slice(0, 18) + '…' : log.incidentId}
        </td>
        <td className="px-4 py-3 text-xs">
          {log.responseStatus ? (
            <span className={`font-mono font-semibold ${log.responseStatus < 300 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
              HTTP {log.responseStatus}
            </span>
          ) : <span className="text-slate-400">—</span>}
        </td>
        <td className="px-4 py-3 max-w-[200px]">
          {log.lastError ? (
            <span className="text-xs text-red-600 dark:text-red-400 truncate block" title={log.lastError}>
              {log.lastError.length > 50 ? log.lastError.slice(0, 50) + '…' : log.lastError}
            </span>
          ) : <span className="text-xs text-slate-400">—</span>}
        </td>
        <td className="px-4 py-3 whitespace-nowrap text-xs text-slate-500">{fmtTime(log.createdAt)}</td>
        <td className="px-4 py-3 whitespace-nowrap text-xs text-slate-500">{fmtTime(log.sentAt)}</td>
      </tr>

      {open && (
        <tr className="bg-slate-50 dark:bg-slate-900/50">
          <td colSpan={7} className="px-6 py-4">
            <div className="grid gap-4 lg:grid-cols-2">
              {/* Request */}
              <div>
                <div className="mb-1.5 flex items-center gap-2">
                  <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Request</span>
                  <span className="text-[10px] text-slate-400">(curl equivalent)</span>
                </div>
                <pre className="overflow-auto rounded-lg border border-slate-200 bg-white p-3 font-mono text-[11px] leading-relaxed text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 max-h-60">
                  {buildCurl(log)}
                </pre>
              </div>

              {/* Response */}
              <div>
                <div className="mb-1.5 flex items-center gap-2">
                  <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Response</span>
                  {log.responseStatus && (
                    <span className={`rounded px-1.5 py-0.5 font-mono text-[10px] font-bold ${
                      log.responseStatus < 300
                        ? 'bg-green-100 text-green-700 dark:bg-green-950/50 dark:text-green-400'
                        : 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-400'
                    }`}>
                      {log.responseStatus}
                    </span>
                  )}
                </div>
                <pre className="overflow-auto rounded-lg border border-slate-200 bg-white p-3 font-mono text-[11px] leading-relaxed text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 max-h-60">
                  {log.responseBody ? prettyJson(log.responseBody) : log.lastError || '(no response body)'}
                </pre>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  )
}

function DeliveryLogs({ channels }: { channels: NotificationChannel[] }) {
  const [statusFilter, setStatusFilter] = useState('')
  const [channelFilter, setChannelFilter] = useState('')
  const [logLimit, setLogLimit] = useState(100)
  const queryClient = useQueryClient()

  const { data: logs, isLoading, error } = useQuery({
    queryKey: ['notification-logs', statusFilter, channelFilter, logLimit],
    queryFn: () => fetchNotificationLogs(statusFilter, channelFilter, logLimit),
    refetchInterval: 15_000,
  })

  if (isLoading) return <LoadingState message="Loading delivery logs…" />
  if (error) return <ErrorState message="Failed to load logs" retry={() => queryClient.invalidateQueries({ queryKey: ['notification-logs'] })} />

  return (
    <div className="space-y-4">
      {/* Filter row */}
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
        {/* Status filter */}
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">Status:</span>
          {(['', 'sent', 'failed', 'pending'] as const).map(s => (
            <button
              key={s}
              onClick={() => setStatusFilter(s)}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                statusFilter === s
                  ? 'bg-blue-600 text-white'
                  : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700'
              }`}
            >
              {s === '' ? 'All' : s}
            </button>
          ))}
        </div>

        {/* Channel filter */}
        {channels.length > 0 && (
          <div className="flex items-center gap-2">
            <span className="text-xs font-medium text-slate-500 dark:text-slate-400">Channel:</span>
            <button
              onClick={() => setChannelFilter('')}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                channelFilter === ''
                  ? 'bg-blue-600 text-white'
                  : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700'
              }`}
            >
              All
            </button>
            {channels.map(ch => (
              <button
                key={ch.id}
                onClick={() => setChannelFilter(channelFilter === ch.name ? '' : ch.name)}
                className={`flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                  channelFilter === ch.name
                    ? 'bg-blue-600 text-white'
                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700'
                }`}
              >
                {ch.name}
              </button>
            ))}
          </div>
        )}

        <span className="ml-auto text-xs text-slate-400">{logs?.length ?? 0} entries · click row to expand</span>
      </div>

      {!logs?.length ? (
        <EmptyState
          icon={<Bell className="h-6 w-6" />}
          title="No delivery logs"
          description="Notification events will appear here once alerts are triggered"
        />
      ) : (
        <>
          <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
            <table className="w-full text-sm">
              <thead className="border-b border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">Status</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">Channel</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">Incident</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">HTTP</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">Error</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">Created</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">Sent At</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {logs.map(log => (
                  <LogRow key={log.notificationId} log={log} />
                ))}
              </tbody>
            </table>
          </div>
          {logs.length >= logLimit && (
            <div className="flex justify-center pt-2">
              <button
                onClick={() => setLogLimit(prev => prev + 100)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800"
              >
                Load more
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

export default function NotificationChannels() {
  const { isAdmin } = useAuth()
  const toast = useToast()
  const confirm = useConfirm()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<'channels' | 'logs'>('channels')
  const [showForm, setShowForm] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<Partial<NotificationChannel>>(emptyForm)

  const { data: channels, isLoading, error } = useQuery({
    queryKey: ['notification-channels'],
    queryFn: fetchChannels,
  })

  const { data: checks } = useQuery({
    queryKey: ['settings', 'checks'],
    queryFn: checksApi.list,
  })

  const saveMutation = useMutation({
    mutationFn: async (data: Partial<NotificationChannel>) => {
      const url = editId ? `/api/v1/notification-channels/${editId}` : '/api/v1/notification-channels'
      const res = await fetch(url, {
        method: editId ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify(data),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body?.error?.message || 'Failed to save channel')
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notification-channels'] })
      setShowForm(false)
      setEditId(null)
      setForm(emptyForm)
      toast.success(editId ? 'Channel updated' : 'Channel created')
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const toggleMutation = useMutation({
    mutationFn: async ({ id, enabled }: { id: string; enabled: boolean }) => {
      const res = await fetch(`/api/v1/notification-channels/${id}/toggle`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ enabled }),
      })
      if (!res.ok) throw new Error('Failed to toggle channel')
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notification-channels'] }),
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`/api/v1/notification-channels/${id}`, {
        method: 'DELETE',
        headers: authHeaders(),
      })
      if (!res.ok) throw new Error('Failed to delete channel')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notification-channels'] })
      toast.success('Channel deleted')
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const testMutation = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch('/api/v1/notification-channels/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ channelId: id }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body?.error?.message || 'Test failed')
      }
    },
    onSuccess: () => toast.success('Test notification sent'),
    onError: (err: Error) => toast.error(err.message),
  })

  const handleSave = () => {
    const validationError = validateChannelForm(form)
    if (validationError) {
      toast.error(validationError)
      return
    }
    saveMutation.mutate(form)
  }

  if (isLoading) return <LoadingState message="Loading channels…" />
  if (error) return <ErrorState message="Failed to load channels" retry={() => { queryClient.invalidateQueries({ queryKey: ['notification-channels'] }) }} />

  const channelIcon = (type: string) => {
    const ct = CHANNEL_TYPES.find(c => c.value === type)
    return ct ? <ct.icon className="h-4 w-4" /> : <Bell className="h-4 w-4" />
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Notification Channels</h1>
          <p className="text-sm text-slate-500">Configure where alerts are sent</p>
        </div>
        {isAdmin && activeTab === 'channels' && (
          <button
            onClick={() => { setShowForm(true); setEditId(null); setForm({ ...emptyForm }) }}
            className="flex items-center gap-2 rounded-lg bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-700"
          >
            <Plus className="h-4 w-4" /> Add Channel
          </button>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-slate-200 dark:border-slate-800">
        <button
          onClick={() => setActiveTab('channels')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'channels'
              ? 'border-blue-600 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
          }`}
        >
          Channels
        </button>
        <button
          onClick={() => setActiveTab('logs')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'logs'
              ? 'border-blue-600 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
          }`}
        >
          Delivery Logs
        </button>
      </div>

      {activeTab === 'logs' && <DeliveryLogs channels={channels ?? []} />}

      {activeTab === 'channels' && (
        <>
          {/* Form */}
          {(showForm || editId) && isAdmin && (
            <div className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
              <h3 className="mb-4 text-sm font-medium text-slate-900 dark:text-slate-100">
                {editId ? 'Edit Channel' : 'New Channel'}
              </h3>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Name</label>
                  <input
                    value={form.name || ''}
                    onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                    placeholder="e.g. Ops Slack"
                    required
                    className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Type</label>
                  <select
                    value={form.type || 'webhook'}
                    onChange={e => setForm(f => ({ ...f, type: e.target.value }))}
                    className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    {CHANNEL_TYPES.map(ct => (
                      <option key={ct.value} value={ct.value}>{ct.label}</option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Type-specific fields */}
              <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                {(form.type === 'webhook' || form.type === 'slack' || form.type === 'discord') && (
                  <div className="sm:col-span-2">
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Webhook URL</label>
                    <input
                    value={form.webhookUrl || ''}
                    onChange={e => setForm(f => ({ ...f, webhookUrl: e.target.value }))}
                    placeholder="https://hooks.slack.com/services/..."
                    type="url"
                    required
                    className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  />
                  </div>
                )}

                {form.type === 'webhook' && (
                  <div className="sm:col-span-2">
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">
                      Body Template <span className="text-slate-400 font-normal">(optional — leave empty to send default payload)</span>
                    </label>
                    <textarea
                      value={form.bodyTemplate || ''}
                      onChange={e => setForm(f => ({ ...f, bodyTemplate: e.target.value }))}
                      rows={6}
                      placeholder={`{\n  "from": "alerts@example.com",\n  "to": "ops@example.com",\n  "subject": "[Alert] {{.CheckName}} - {{.Severity}}",\n  "html": "<h2>{{.Message}}</h2><p>Check: {{.CheckName}} | Server: {{.Server}}</p>"\n}`}
                      className="w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    />
                    <p className="mt-1 text-xs text-slate-400">
                      Available fields: <code className="text-slate-500">{`{{.CheckName}} {{.Severity}} {{.Status}} {{.Message}} {{.Server}} {{.CheckID}} {{.IncidentID}} {{.StartedAt}} {{.ResolvedAt}}`}</code>
                    </p>
                  </div>
                )}

                {form.type === 'email' && (
                  <>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Recipients (comma-separated)</label>
                      <input
                        value={form.email || ''}
                        onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
                        placeholder="ops@example.com, admin@example.com"
                        required
                        className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">From Email</label>
                      <input
                        value={form.fromEmail || ''}
                        onChange={e => setForm(f => ({ ...f, fromEmail: e.target.value }))}
                        placeholder="healthops@example.com"
                        type="email"
                        required
                        className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">SMTP Host</label>
                      <input
                        value={form.smtpHost || ''}
                        onChange={e => setForm(f => ({ ...f, smtpHost: e.target.value }))}
                        placeholder="smtp.gmail.com"
                        required
                        className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">SMTP Port</label>
                      <input
                        type="number"
                        value={form.smtpPort || 587}
                        onChange={e => setForm(f => ({ ...f, smtpPort: parseInt(e.target.value) || 587 }))}
                        min={1}
                        max={65535}
                        required
                        className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">SMTP Username</label>
                      <input
                        value={form.smtpUser || ''}
                        onChange={e => setForm(f => ({ ...f, smtpUser: e.target.value }))}
                        className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      />
                    </div>
                  </>
                )}

                {form.type === 'telegram' && (
                  <>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Bot Token</label>
                      <input
                        type="password"
                        autoComplete="new-password"
                        value={form.botToken || ''}
                        onChange={e => setForm(f => ({ ...f, botToken: e.target.value }))}
                        placeholder="123456:ABC-DEF..."
                        required
                        className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Chat ID</label>
                      <input
                        value={form.chatId || ''}
                        onChange={e => setForm(f => ({ ...f, chatId: e.target.value }))}
                        placeholder="-1001234567890"
                        required
                        className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      />
                    </div>
                  </>
                )}

                {form.type === 'pagerduty' && (
                  <div className="sm:col-span-2">
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Routing Key</label>
                    <input
                      value={form.routingKey || ''}
                      onChange={e => setForm(f => ({ ...f, routingKey: e.target.value }))}
                      placeholder="PagerDuty integration routing key"
                      required
                      className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    />
                  </div>
                )}
              </div>

              {/* Options */}
              <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Cooldown (minutes)</label>
                  <input
                    type="number"
                    value={form.cooldownMinutes || 5}
                    onChange={e => setForm(f => ({ ...f, cooldownMinutes: parseInt(e.target.value) || 5 }))}
                    className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Severity Filter</label>
                  <div className="flex flex-wrap gap-2 pt-1">
                    {['critical', 'warning', 'info'].map(s => (
                      <label key={s} className="flex items-center gap-1 text-xs text-slate-600 dark:text-slate-400">
                        <input
                          type="checkbox"
                          checked={form.severities?.includes(s) ?? false}
                          onChange={e => {
                            setForm(f => ({
                              ...f,
                              severities: e.target.checked
                                ? [...(f.severities || []), s]
                                : (f.severities || []).filter(x => x !== s),
                            }))
                          }}
                          className="rounded"
                        />
                        {s}
                      </label>
                    ))}
                  </div>
                </div>
                <div className="flex items-end">
                  <label className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-400">
                    <input
                      type="checkbox"
                      checked={form.notifyOnResolve ?? true}
                      onChange={e => setForm(f => ({ ...f, notifyOnResolve: e.target.checked }))}
                      className="rounded"
                    />
                    Notify on resolve
                  </label>
                </div>
              </div>

              {/* Smart Filters */}
              <div className="mt-4 rounded-lg border border-slate-200 p-4 dark:border-slate-700">
                <div className="mb-3 flex items-center gap-2 text-xs font-medium text-slate-700 dark:text-slate-300">
                  <Filter className="h-3.5 w-3.5" /> Routing Filters
                  <span className="text-slate-400 font-normal">(empty = send for all)</span>
                </div>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Health Checks</label>
                    <div className="max-h-32 overflow-y-auto rounded border border-slate-200 p-2 dark:border-slate-700">
                      {(checks ?? []).map(c => (
                        <label key={c.id} className="flex items-center gap-2 py-0.5 text-xs text-slate-600 dark:text-slate-400">
                          <input
                            type="checkbox"
                            checked={form.checkIds?.includes(c.id) ?? false}
                            onChange={e => {
                              setForm(f => ({
                                ...f,
                                checkIds: e.target.checked
                                  ? [...(f.checkIds || []), c.id]
                                  : (f.checkIds || []).filter(x => x !== c.id),
                              }))
                            }}
                            className="rounded"
                          />
                          <span className="truncate">{c.name}</span>
                          <span className="ml-auto text-[10px] text-slate-400">{c.type}</span>
                        </label>
                      ))}
                      {!(checks?.length) && <p className="text-xs text-slate-400 italic">No checks configured</p>}
                    </div>
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Check Types</label>
                    <div className="flex flex-wrap gap-2 pt-1">
                      {['api', 'tcp', 'process', 'command', 'log', 'mysql', 'ssh'].map(t => (
                        <label key={t} className="flex items-center gap-1 text-xs text-slate-600 dark:text-slate-400">
                          <input
                            type="checkbox"
                            checked={form.checkTypes?.includes(t) ?? false}
                            onChange={e => {
                              setForm(f => ({
                                ...f,
                                checkTypes: e.target.checked
                                  ? [...(f.checkTypes || []), t]
                                  : (f.checkTypes || []).filter(x => x !== t),
                              }))
                            }}
                            className="rounded"
                          />
                          {t}
                        </label>
                      ))}
                    </div>
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Servers</label>
                    <input
                      value={(form.servers || []).join(', ')}
                      onChange={e => setForm(f => ({ ...f, servers: e.target.value.split(',').map(s => s.trim()).filter(Boolean) }))}
                      placeholder="production, staging"
                      className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Tags</label>
                    <input
                      value={(form.tags || []).join(', ')}
                      onChange={e => setForm(f => ({ ...f, tags: e.target.value.split(',').map(s => s.trim()).filter(Boolean) }))}
                      placeholder="critical, database"
                      className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    />
                  </div>
                </div>
              </div>

              <div className="mt-4 flex gap-2">
                <button
                  onClick={handleSave}
                  disabled={saveMutation.isPending}
                  className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  {editId ? 'Update' : 'Create'}
                </button>
                <button
                  onClick={() => { setShowForm(false); setEditId(null); setForm(emptyForm) }}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-400"
                >
                  Cancel
                </button>
              </div>
            </div>
          )}

          {/* Channel List */}
          {!channels?.length ? (
            <EmptyState icon={<Bell className="h-6 w-6" />} title="No channels configured" description="Add a notification channel to receive alerts via email, Slack, webhooks, and more" />
          ) : (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {channels.map(ch => (
                <div key={ch.id} className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
                  <div className="flex items-start justify-between">
                    <div className="flex items-center gap-3">
                      <div className={`flex h-9 w-9 items-center justify-center rounded-lg ${ch.enabled ? 'bg-blue-100 text-blue-600 dark:bg-blue-950/50 dark:text-blue-400' : 'bg-slate-100 text-slate-400 dark:bg-slate-800'}`}>
                        {channelIcon(ch.type)}
                      </div>
                      <div>
                        <h3 className="text-sm font-medium text-slate-900 dark:text-slate-100">{ch.name}</h3>
                        <p className="text-xs text-slate-500 capitalize">{ch.type}</p>
                      </div>
                    </div>
                    {isAdmin && (
                      <button
                        onClick={() => toggleMutation.mutate({ id: ch.id, enabled: !ch.enabled })}
                        className="text-slate-400 hover:text-slate-600"
                        title={ch.enabled ? 'Disable' : 'Enable'}
                      >
                        {ch.enabled ? <ToggleRight className="h-5 w-5 text-blue-600" /> : <ToggleLeft className="h-5 w-5" />}
                      </button>
                    )}
                  </div>

                  {/* Details */}
                  <div className="mt-3 space-y-1 text-xs text-slate-500">
                    {ch.webhookUrl && <p className="truncate">URL: {ch.webhookUrl}</p>}
                    {ch.email && <p className="truncate">To: {ch.email}</p>}
                    {ch.chatId && <p>Chat: {ch.chatId}</p>}
                    {ch.severities && ch.severities.length > 0 && (
                      <p>Severities: {ch.severities.join(', ')}</p>
                    )}
                    {ch.checkIds && ch.checkIds.length > 0 && (
                      <p>Checks: {ch.checkIds.length} selected</p>
                    )}
                    {ch.checkTypes && ch.checkTypes.length > 0 && (
                      <p>Types: {ch.checkTypes.join(', ')}</p>
                    )}
                    {ch.servers && ch.servers.length > 0 && (
                      <p>Servers: {ch.servers.join(', ')}</p>
                    )}
                    {ch.tags && ch.tags.length > 0 && (
                      <p>Tags: {ch.tags.join(', ')}</p>
                    )}
                    {ch.cooldownMinutes ? <p>Cooldown: {ch.cooldownMinutes}min</p> : null}
                  </div>

                  {/* Actions */}
                  {isAdmin && (
                    <div className="mt-3 flex gap-1 border-t border-slate-100 pt-3 dark:border-slate-800">
                      <button
                        onClick={() => testMutation.mutate(ch.id)}
                        className="flex items-center gap-1 rounded px-2 py-1 text-xs text-slate-500 hover:bg-slate-50 hover:text-blue-600 dark:hover:bg-slate-800"
                        disabled={testMutation.isPending}
                      >
                        <Send className="h-3 w-3" /> Test
                      </button>
                      <button
                        onClick={() => {
                          setEditId(ch.id)
                          setShowForm(false)
                          setForm({ ...ch })
                        }}
                        className="flex items-center gap-1 rounded px-2 py-1 text-xs text-slate-500 hover:bg-slate-50 hover:text-blue-600 dark:hover:bg-slate-800"
                      >
                        <Pencil className="h-3 w-3" /> Edit
                      </button>
                      <button
                        onClick={async () => {
                          const ok = await confirm({
                            title: 'Delete Notification Channel',
                            message: `Delete channel "${ch.name}"? Alerts routed only to this channel will stop sending.`,
                            confirmLabel: 'Delete',
                            variant: 'danger',
                          })
                          if (ok) deleteMutation.mutate(ch.id)
                        }}
                        className="flex items-center gap-1 rounded px-2 py-1 text-xs text-slate-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/50"
                      >
                        <Trash2 className="h-3 w-3" /> Delete
                      </button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}
