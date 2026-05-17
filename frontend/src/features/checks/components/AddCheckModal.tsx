import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Wrench } from 'lucide-react'
import { checksApi } from "@/features/checks/api/checks"
import { serversApi } from "@/features/servers/api/servers"
import { checkTypeLabel, cn } from "@/shared/lib/utils"
import { CHECK_TYPES } from "@/shared/lib/constants"
import type { CheckConfig, CheckType } from "@/shared/types"

function deriveTarget(check: CheckConfig): string {
  switch (check.type) {
    case 'api':
    case 'process':
      return check.target ?? ''
    case 'tcp':
      return check.host ? `${check.host}:${check.port ?? ''}` : ''
    case 'command':
      return check.command ?? ''
    case 'log':
      return check.path ?? ''
    case 'mysql':
      return check.mysql?.dsnEnv ?? ''
    case 'ssh':
      return check.ssh ? `${check.ssh.host}:${check.ssh.port ?? 22}` : ''
    case 'ssl':
      return check.ssl?.host ?? check.host ?? check.target ?? ''
    case 'dns':
      return check.dns?.name ?? check.target ?? ''
    case 'ping':
      return check.ping?.host ?? check.host ?? check.target ?? ''
    case 'domain':
      return check.domain?.domain ?? check.target ?? ''
    case 'heartbeat':
      return check.heartbeat?.expectedIntervalSeconds ? String(check.heartbeat.expectedIntervalSeconds) : ''
    default:
      return ''
  }
}

const TARGET_META: Record<CheckType, { label: string; placeholder: string; hint: string }> = {
  api: {
    label: 'URL',
    placeholder: 'https://example.com/healthz',
    hint: 'HTTP GET endpoint expected to return a healthy status.',
  },
  tcp: {
    label: 'Host and Port',
    placeholder: 'mysql.internal:3306',
    hint: 'Use host:port so the backend can create TCP host and port fields.',
  },
  process: {
    label: 'Process Name',
    placeholder: 'nginx',
    hint: 'Substring matched against the process list.',
  },
  command: {
    label: 'Command',
    placeholder: '/usr/local/bin/check-backup.sh',
    hint: 'Command must exit 0 when healthy. Command checks must be enabled server-side.',
  },
  log: {
    label: 'Log Path',
    placeholder: '/var/log/app.log',
    hint: 'File must exist and be fresh according to backend freshness settings.',
  },
  mysql: {
    label: 'DSN Environment Variable',
    placeholder: 'MYSQL_DSN',
    hint: 'Name of an environment variable containing the MySQL DSN.',
  },
  ssh: {
    label: 'SSH Host and Port',
    placeholder: 'linux-server-1:22',
    hint: 'Use host:port. Provide the SSH user below.',
  },
  ssl: {
    label: 'Host or URL',
    placeholder: 'https://example.com',
    hint: 'Checks certificate validity and expiry. URLs are reduced to their host automatically.',
  },
  dns: {
    label: 'DNS Name',
    placeholder: 'example.com',
    hint: 'Creates an A-record DNS check. Advanced record expectations can be edited through the API.',
  },
  ping: {
    label: 'Host or IP',
    placeholder: '10.0.0.10',
    hint: 'Runs reachability probes and tracks packet loss/latency.',
  },
  domain: {
    label: 'Domain',
    placeholder: 'example.com',
    hint: 'Checks domain expiry through RDAP, with WHOIS fallback when available.',
  },
  heartbeat: {
    label: 'Expected Interval (seconds)',
    placeholder: '300',
    hint: 'Creates a push heartbeat. The generated token is available on the check and heartbeat APIs.',
  },
}

function splitHostPort(value: string, defaultPort?: number) {
  const trimmed = value.trim()
  const lastColon = trimmed.lastIndexOf(':')
  if (lastColon <= 0) {
    return { host: trimmed, port: defaultPort }
  }

  const host = trimmed.slice(0, lastColon)
  const port = Number(trimmed.slice(lastColon + 1))
  return { host, port: Number.isInteger(port) && port > 0 ? port : defaultPort }
}

function buildCheckPayload(input: {
  name: string
  type: CheckType
  server: string
  serverId: string
  target: string
  enabled: boolean
  sshUser: string
  mysqlMode: 'inline' | 'env'
  mysqlInline: { host: string; port: string; username: string; password: string; database: string }
  remediation?: { actionRef: string; maxAttempts: string; cooldownSeconds: string; risk: '' | 'low' | 'medium' | 'high' }
}): Partial<CheckConfig> {
  const name = input.name.trim()
  const server = input.server.trim()
  const serverId = input.serverId.trim()
  const target = input.target.trim()
  const base: Partial<CheckConfig> = {
    name,
    type: input.type,
    server: server || undefined,
    serverId: serverId || undefined,
    enabled: input.enabled,
  }

  // Attach optional remediation (only when serverId is set + at least one remediation field provided)
  if (serverId && input.remediation) {
    const r = input.remediation
    const actionRef = r.actionRef.trim()
    const maxAttemptsNum = r.maxAttempts.trim() ? Number(r.maxAttempts) : undefined
    const cooldownNum = r.cooldownSeconds.trim() ? Number(r.cooldownSeconds) : undefined
    if (actionRef || maxAttemptsNum || cooldownNum || r.risk) {
      const remediation: NonNullable<CheckConfig['remediation']> = {}
      if (actionRef) remediation.actionRef = actionRef
      if (maxAttemptsNum && Number.isInteger(maxAttemptsNum) && maxAttemptsNum > 0) remediation.maxAttempts = maxAttemptsNum
      if (cooldownNum && Number.isFinite(cooldownNum) && cooldownNum >= 0) remediation.cooldownSeconds = cooldownNum
      if (r.risk) remediation.risk = r.risk
      base.remediation = remediation
    }
  }

  switch (input.type) {
    case 'api':
    case 'process':
      return { ...base, target }
    case 'tcp': {
      const { host, port } = splitHostPort(target)
      return { ...base, host, port }
    }
    case 'command':
      return { ...base, command: target }
    case 'log':
      return { ...base, path: target }
    case 'mysql': {
      if (input.mysqlMode === 'env') {
        return { ...base, mysql: { dsnEnv: target } }
      }
      const inline = input.mysqlInline
      const portNum = inline.port.trim() ? Number(inline.port) : undefined
      const mysql: NonNullable<CheckConfig['mysql']> = {
        host: inline.host.trim(),
        username: inline.username.trim(),
      }
      if (portNum && Number.isInteger(portNum) && portNum > 0) mysql.port = portNum
      if (inline.database.trim()) mysql.database = inline.database.trim()
      if (inline.password) mysql.password = inline.password
      return { ...base, mysql }
    }
    case 'ssh': {
      const { host, port } = splitHostPort(target, 22)
      return { ...base, ssh: { host, port, user: input.sshUser.trim() } }
    }
    case 'ssl':
      return { ...base, target, ssl: { host: target } }
    case 'dns':
      return { ...base, target, dns: { name: target, recordType: 'A' } }
    case 'ping':
      return { ...base, target, ping: { host: target } }
    case 'domain':
      return { ...base, target, domain: { domain: target } }
    case 'heartbeat':
      return { ...base, heartbeat: { expectedIntervalSeconds: Number(target), graceSeconds: 60 } }
  }
}

function validatePayload(input: {
  name: string
  type: CheckType
  target: string
  sshUser: string
  mysqlMode: 'inline' | 'env'
  mysqlInline: { host: string; username: string }
}) {
  if (!input.name.trim()) return 'Check name is required'
  if (input.type === 'mysql') {
    if (input.mysqlMode === 'env') {
      if (!input.target.trim()) return 'DSN environment variable name is required'
    } else {
      if (!input.mysqlInline.host.trim()) return 'MySQL host is required'
      if (!input.mysqlInline.username.trim()) return 'MySQL username is required'
    }
  } else if (!input.target.trim()) {
    return `${TARGET_META[input.type].label} is required`
  }
  if (input.type === 'tcp') {
    const { host, port } = splitHostPort(input.target)
    if (!host || !port) return 'TCP target must be host:port'
  }
  if (input.type === 'ssh') {
    const { host, port } = splitHostPort(input.target, 22)
    if (!host || !port) return 'SSH target must include a host'
    if (!input.sshUser.trim()) return 'SSH user is required'
  }
  if (input.type === 'heartbeat') {
    const interval = Number(input.target.trim())
    if (!Number.isInteger(interval) || interval <= 0) return 'Heartbeat interval must be a positive number of seconds'
  }
  return null
}

export function AddCheckModal({
  defaultServer,
  defaultType,
  initialData,
  onClose,
  onCreated,
}: {
  defaultServer?: string
  defaultType?: CheckType
  initialData?: CheckConfig
  onClose: () => void
  onCreated: () => void
}) {
  const queryClient = useQueryClient()
  const isEditing = initialData != null

  const [name, setName] = useState(initialData?.name ?? '')
  const [type, setType] = useState<CheckType>(initialData?.type ?? defaultType ?? 'api')
  const [server, setServer] = useState(initialData?.server ?? defaultServer ?? '')
  const [serverId, setServerId] = useState(initialData?.serverId ?? '')
  const [target, setTarget] = useState(initialData ? deriveTarget(initialData) : '')
  const [sshUser, setSshUser] = useState(initialData?.ssh?.user ?? 'root')
  const [enabled, setEnabled] = useState(initialData?.enabled ?? true)
  // MySQL credential mode: 'inline' (UI form, encrypted at rest) vs 'env' (env var DSN, advanced)
  const initialMysqlMode: 'inline' | 'env' = initialData?.mysql?.dsnEnv ? 'env' : 'inline'
  const [mysqlMode, setMysqlMode] = useState<'inline' | 'env'>(initialMysqlMode)
  const [mysqlHost, setMysqlHost] = useState(initialData?.mysql?.host ?? '')
  const [mysqlPort, setMysqlPort] = useState(initialData?.mysql?.port ? String(initialData.mysql.port) : '3306')
  const [mysqlUsername, setMysqlUsername] = useState(initialData?.mysql?.username ?? '')
  const [mysqlPassword, setMysqlPassword] = useState(initialData?.mysql?.password ?? '')
  const [mysqlDatabase, setMysqlDatabase] = useState(initialData?.mysql?.database ?? '')
  // Remediation overrides (visible when a server is selected). Edits use the full editor page.
  const [remediationActionRef, setRemediationActionRef] = useState(initialData?.remediation?.actionRef ?? '')
  const [remediationMaxAttempts, setRemediationMaxAttempts] = useState(
    initialData?.remediation?.maxAttempts ? String(initialData.remediation.maxAttempts) : ''
  )
  const [remediationCooldown, setRemediationCooldown] = useState(
    initialData?.remediation?.cooldownSeconds ? String(initialData.remediation.cooldownSeconds) : ''
  )
  const [remediationRisk, setRemediationRisk] = useState<'' | 'low' | 'medium' | 'high'>(
    initialData?.remediation?.risk ?? ''
  )
  const [validationError, setValidationError] = useState<string | null>(null)
  const [createdHeartbeat, setCreatedHeartbeat] = useState<CheckConfig | null>(null)

  const { data: servers = [] } = useQuery({
    queryKey: ['servers'],
    queryFn: () => serversApi.list(),
    staleTime: 30_000,
  })

  const handleServerChange = (value: string) => {
    // value is either a server.id (when picked from registry) or '' for None.
    if (!value) {
      setServerId('')
      setServer('')
      return
    }
    const match = servers.find(s => s.id === value)
    if (match) {
      setServerId(match.id)
      setServer(match.name)
    }
  }

  const mutation = useMutation({
    mutationFn: (check: Partial<CheckConfig>) =>
      isEditing ? checksApi.update(initialData!.id, check) : checksApi.create(check),
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: ['checks'] })
      if (!isEditing && created.type === 'heartbeat' && created.heartbeat?.token) {
        setCreatedHeartbeat(created)
        return
      }
      onCreated()
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setValidationError(null)

    const mysqlInline = {
      host: mysqlHost,
      port: mysqlPort,
      username: mysqlUsername,
      password: mysqlPassword,
      database: mysqlDatabase,
    }

    const error = validatePayload({ name, type, target, sshUser, mysqlMode, mysqlInline })
    if (error) {
      setValidationError(error)
      return
    }

    mutation.mutate(buildCheckPayload({
      name,
      type,
      server,
      serverId,
      target,
      sshUser,
      enabled,
      mysqlMode,
      mysqlInline,
      remediation: {
        actionRef: remediationActionRef,
        maxAttempts: remediationMaxAttempts,
        cooldownSeconds: remediationCooldown,
        risk: remediationRisk,
      },
    }))
  }

  const targetMeta = TARGET_META[type]
  const heartbeatURL = createdHeartbeat?.heartbeat?.token
    ? `${window.location.origin}/api/v1/heartbeats/${createdHeartbeat.heartbeat.token}`
    : ''

  if (createdHeartbeat && heartbeatURL) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
        <div
          className="w-full max-w-lg rounded-xl bg-white p-6 shadow-xl dark:bg-slate-800"
          onClick={(e) => e.stopPropagation()}
        >
          <h2 className="mb-2 text-lg font-semibold text-slate-900 dark:text-white">Heartbeat Created</h2>
          <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
            Store this ping URL in the job or cron task that should report healthy execution.
          </p>
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-900">
            <div className="mb-1 text-xs font-medium uppercase tracking-wide text-slate-500">Ping URL</div>
            <code className="block break-all text-xs text-slate-800 dark:text-slate-200">{heartbeatURL}</code>
          </div>
          <div className="mt-4 grid gap-2 sm:grid-cols-2">
            <button
              type="button"
              onClick={() => navigator.clipboard.writeText(heartbeatURL)}
              className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-300 dark:hover:bg-slate-700"
            >
              Copy URL
            </button>
            <button
              type="button"
              onClick={() => onCreated()}
              className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
            >
              Done
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-xl bg-white p-6 shadow-xl dark:bg-slate-800"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-lg font-semibold text-slate-900 dark:text-white">{isEditing ? 'Edit Check' : 'Add Check'}</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">Name</label>
            <input
              type="text"
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My health check"
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white dark:placeholder:text-slate-500"
            />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">Type</label>
            <select
              value={type}
              onChange={(e) => setType(e.target.value as CheckType)}
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white"
            >
              {CHECK_TYPES.map((t) => (
                <option key={t} value={t}>{checkTypeLabel(t)}</option>
              ))}
            </select>
          </div>

          <div>
            <div className="mb-1 flex items-center justify-between">
              <label className="block text-sm font-medium text-slate-700 dark:text-slate-300">Server</label>
              <Link
                to="/servers"
                onClick={onClose}
                className="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400"
              >
                + Add server
              </Link>
            </div>
            {servers.length > 0 ? (
              <>
                <select
                  value={serverId}
                  onChange={(e) => handleServerChange(e.target.value)}
                  aria-label="Server"
                  className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white"
                >
                  <option value="">— None —</option>
                  {servers.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.host}{s.port && s.port !== 22 ? `:${s.port}` : ''})
                    </option>
                  ))}
                  {!serverId && server && (
                    // Legacy free-text label that doesn't match a registered server
                    <option value="" disabled>— Legacy label: {server} —</option>
                  )}
                </select>
                <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  Pick a registered server so auto-remediation can SSH into it. Manage servers under Servers.
                </p>
              </>
            ) : (
              <>
                <input
                  type="text"
                  value={server}
                  onChange={(e) => { setServer(e.target.value); setServerId('') }}
                  placeholder="server name"
                  className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white dark:placeholder:text-slate-500"
                />
                <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  No servers registered yet. Add one under Servers to enable SSH-based remediation.
                </p>
              </>
            )}
          </div>

          {type === 'mysql' ? (
            <MySQLCredentialsSection
              mode={mysqlMode}
              setMode={setMysqlMode}
              host={mysqlHost}
              setHost={setMysqlHost}
              port={mysqlPort}
              setPort={setMysqlPort}
              username={mysqlUsername}
              setUsername={setMysqlUsername}
              password={mysqlPassword}
              setPassword={setMysqlPassword}
              hasStoredPassword={Boolean(initialData?.mysql?.hasPassword)}
              database={mysqlDatabase}
              setDatabase={setMysqlDatabase}
              dsnEnv={target}
              setDsnEnv={setTarget}
            />
          ) : (
            <div>
              <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">{targetMeta.label}</label>
              <input
                type={type === 'heartbeat' ? 'number' : 'text'}
                required
                min={type === 'heartbeat' ? 1 : undefined}
                value={target}
                onChange={(e) => setTarget(e.target.value)}
                placeholder={targetMeta.placeholder}
                className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white dark:placeholder:text-slate-500"
              />
              <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">{targetMeta.hint}</p>
            </div>
          )}

          {type === 'ssh' && (
            <div>
              <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">SSH User</label>
              <input
                type="text"
                required
                value={sshUser}
                onChange={(e) => setSshUser(e.target.value)}
                placeholder="root"
                className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white dark:placeholder:text-slate-500"
              />
            </div>
          )}

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setEnabled(!enabled)}
              className={cn(
                'relative h-5 w-9 rounded-full transition-colors',
                enabled ? 'bg-blue-600' : 'bg-slate-300 dark:bg-slate-600'
              )}
            >
              <span
                className={cn(
                  'absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white transition-transform',
                  enabled && 'translate-x-4'
                )}
              />
            </button>
            <span className="text-sm text-slate-700 dark:text-slate-300">Enabled</span>
          </div>

          {serverId && (
            <RemediationQuickSection
              actionRef={remediationActionRef}
              setActionRef={setRemediationActionRef}
              maxAttempts={remediationMaxAttempts}
              setMaxAttempts={setRemediationMaxAttempts}
              cooldownSeconds={remediationCooldown}
              setCooldownSeconds={setRemediationCooldown}
              risk={remediationRisk}
              setRisk={setRemediationRisk}
              isEditing={isEditing}
            />
          )}

          {(validationError || mutation.isError) && (
            <p className="text-sm text-red-600 dark:text-red-400">
              {validationError || (mutation.error instanceof Error ? mutation.error.message : isEditing ? 'Failed to update check' : 'Failed to create check')}
            </p>
          )}

          {isEditing && (
            <Link
              to={`/checks/${initialData!.id}?edit=1`}
              onClick={onClose}
              className="flex items-center justify-between gap-3 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-3 py-2.5 text-sm text-slate-700 hover:border-blue-400 hover:bg-blue-50 hover:text-blue-700 dark:border-slate-600 dark:bg-slate-900/40 dark:text-slate-300 dark:hover:border-blue-500 dark:hover:bg-blue-950/40 dark:hover:text-blue-300"
            >
              <span className="flex items-center gap-2">
                <Wrench className="h-4 w-4" />
                Auto-remediation, alert rules &amp; channels
              </span>
              <span className="text-xs font-medium">Open full editor →</span>
            </Link>
          )}

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-300 dark:hover:bg-slate-700"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={mutation.isPending}
              className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {mutation.isPending ? (isEditing ? 'Saving...' : 'Creating...') : (isEditing ? 'Save Changes' : 'Create Check')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// --- Inline subcomponents ---

const inputCls =
  'w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white dark:placeholder:text-slate-500'

function MySQLCredentialsSection(props: {
  mode: 'inline' | 'env'
  setMode: (m: 'inline' | 'env') => void
  host: string
  setHost: (v: string) => void
  port: string
  setPort: (v: string) => void
  username: string
  setUsername: (v: string) => void
  password: string
  setPassword: (v: string) => void
  hasStoredPassword: boolean
  database: string
  setDatabase: (v: string) => void
  dsnEnv: string
  setDsnEnv: (v: string) => void
}) {
  const {
    mode, setMode,
    host, setHost, port, setPort, username, setUsername,
    password, setPassword, hasStoredPassword,
    database, setDatabase,
    dsnEnv, setDsnEnv,
  } = props
  return (
    <div className="space-y-3 rounded-lg border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-900/40">
      <div className="flex items-center justify-between gap-2">
        <label className="text-sm font-medium text-slate-700 dark:text-slate-300">MySQL connection</label>
        <div className="inline-flex rounded-md border border-slate-300 bg-white p-0.5 text-xs dark:border-slate-600 dark:bg-slate-800">
          <button
            type="button"
            onClick={() => setMode('inline')}
            className={cn(
              'rounded px-2 py-1 font-medium',
              mode === 'inline' ? 'bg-blue-600 text-white' : 'text-slate-600 dark:text-slate-300'
            )}
          >
            Inline (recommended)
          </button>
          <button
            type="button"
            onClick={() => setMode('env')}
            className={cn(
              'rounded px-2 py-1 font-medium',
              mode === 'env' ? 'bg-blue-600 text-white' : 'text-slate-600 dark:text-slate-300'
            )}
          >
            Env variable
          </button>
        </div>
      </div>

      {mode === 'inline' ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Host</label>
            <input type="text" required value={host} onChange={(e) => setHost(e.target.value)} placeholder="mysql.example.com" className={inputCls} />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Port</label>
            <input type="number" min={1} max={65535} value={port} onChange={(e) => setPort(e.target.value)} placeholder="3306" className={inputCls} />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Database <span className="text-slate-400">(optional)</span></label>
            <input type="text" value={database} onChange={(e) => setDatabase(e.target.value)} placeholder="appdb" className={inputCls} />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Username</label>
            <input type="text" required value={username} onChange={(e) => setUsername(e.target.value)} placeholder="monitor" className={inputCls} />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={hasStoredPassword ? '******** (leave blank to keep)' : 'password'}
              autoComplete="new-password"
              className={inputCls}
            />
            {hasStoredPassword && !password && (
              <p className="mt-1 text-[11px] text-emerald-700 dark:text-emerald-400">✓ Password stored — encrypted at rest</p>
            )}
          </div>
          <p className="sm:col-span-2 text-[11px] text-slate-500 dark:text-slate-400">
            Credentials are encrypted at rest using AES-256-GCM and never returned in plaintext via the API.
          </p>
        </div>
      ) : (
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">DSN environment variable name</label>
          <input
            type="text"
            required
            value={dsnEnv}
            onChange={(e) => setDsnEnv(e.target.value)}
            placeholder="MYSQL_DSN"
            className={inputCls}
          />
          <p className="mt-1 text-[11px] text-slate-500 dark:text-slate-400">
            The backend reads the DSN from the named environment variable. Use this when secrets are managed by your orchestrator.
          </p>
        </div>
      )}
    </div>
  )
}

function RemediationQuickSection(props: {
  actionRef: string
  setActionRef: (v: string) => void
  maxAttempts: string
  setMaxAttempts: (v: string) => void
  cooldownSeconds: string
  setCooldownSeconds: (v: string) => void
  risk: '' | 'low' | 'medium' | 'high'
  setRisk: (v: '' | 'low' | 'medium' | 'high') => void
  isEditing: boolean
}) {
  const { actionRef, setActionRef, maxAttempts, setMaxAttempts, cooldownSeconds, setCooldownSeconds, risk, setRisk, isEditing } = props
  return (
    <div className="space-y-3 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-900/40 dark:bg-amber-950/20">
      <div className="flex items-center justify-between gap-2">
        <label className="text-sm font-medium text-amber-900 dark:text-amber-200">Auto-remediation <span className="text-xs font-normal">(optional)</span></label>
        <span className="text-[11px] text-amber-700 dark:text-amber-300">Runs over the selected server's SSH</span>
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="sm:col-span-2">
          <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Action reference</label>
          <input
            type="text"
            value={actionRef}
            onChange={(e) => setActionRef(e.target.value)}
            placeholder="restart-nginx"
            className={inputCls}
          />
          <p className="mt-1 text-[11px] text-slate-500 dark:text-slate-400">
            Name of a registered remediation action. Leave blank to skip auto-remediation for now.
          </p>
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Max attempts</label>
          <input
            type="number"
            min={1}
            value={maxAttempts}
            onChange={(e) => setMaxAttempts(e.target.value)}
            placeholder="1"
            className={inputCls}
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Cooldown (seconds)</label>
          <input
            type="number"
            min={0}
            value={cooldownSeconds}
            onChange={(e) => setCooldownSeconds(e.target.value)}
            placeholder="300"
            className={inputCls}
          />
        </div>
        <div className="sm:col-span-2">
          <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Risk level</label>
          <select
            value={risk}
            onChange={(e) => setRisk(e.target.value as '' | 'low' | 'medium' | 'high')}
            className={inputCls}
          >
            <option value="">— Not set —</option>
            <option value="low">Low</option>
            <option value="medium">Medium</option>
            <option value="high">High</option>
          </select>
        </div>
      </div>
      <p className="text-[11px] text-amber-700 dark:text-amber-300">
        {isEditing
          ? 'For inline commands, HTTP webhooks, escalation policies and verification timing use the full editor below.'
          : 'For inline commands, HTTP webhooks, escalation policies and verification timing open the check after creation.'}
      </p>
    </div>
  )
}
