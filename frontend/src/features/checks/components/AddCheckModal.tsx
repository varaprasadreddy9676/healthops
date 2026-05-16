import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { checksApi } from "@/features/checks/api/checks"
import { checkTypeLabel, cn } from "@/shared/lib/utils"
import { CHECK_TYPES } from "@/shared/lib/constants"
import type { CheckConfig } from "@/shared/types"

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
    default:
      return ''
  }
}

const TARGET_META: Record<CheckConfig['type'], { label: string; placeholder: string; hint: string }> = {
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
  type: CheckConfig['type']
  server: string
  target: string
  enabled: boolean
  sshUser: string
}): Partial<CheckConfig> {
  const name = input.name.trim()
  const server = input.server.trim()
  const target = input.target.trim()
  const base: Partial<CheckConfig> = {
    name,
    type: input.type,
    server: server || undefined,
    enabled: input.enabled,
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
    case 'mysql':
      return { ...base, mysql: { dsnEnv: target } }
    case 'ssh': {
      const { host, port } = splitHostPort(target, 22)
      return { ...base, ssh: { host, port, user: input.sshUser.trim() } }
    }
  }
}

function validatePayload(input: {
  name: string
  type: CheckConfig['type']
  target: string
  sshUser: string
}) {
  if (!input.name.trim()) return 'Check name is required'
  if (!input.target.trim()) return `${TARGET_META[input.type].label} is required`
  if (input.type === 'tcp') {
    const { host, port } = splitHostPort(input.target)
    if (!host || !port) return 'TCP target must be host:port'
  }
  if (input.type === 'ssh') {
    const { host, port } = splitHostPort(input.target, 22)
    if (!host || !port) return 'SSH target must include a host'
    if (!input.sshUser.trim()) return 'SSH user is required'
  }
  return null
}

export function AddCheckModal({
  defaultServer,
  initialData,
  onClose,
  onCreated,
}: {
  defaultServer?: string
  initialData?: CheckConfig
  onClose: () => void
  onCreated: () => void
}) {
  const queryClient = useQueryClient()
  const isEditing = initialData != null

  const [name, setName] = useState(initialData?.name ?? '')
  const [type, setType] = useState<CheckConfig['type']>(initialData?.type ?? 'api')
  const [server, setServer] = useState(initialData?.server ?? defaultServer ?? '')
  const [target, setTarget] = useState(initialData ? deriveTarget(initialData) : '')
  const [sshUser, setSshUser] = useState(initialData?.ssh?.user ?? 'root')
  const [enabled, setEnabled] = useState(initialData?.enabled ?? true)
  const [validationError, setValidationError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: (check: Partial<CheckConfig>) =>
      isEditing ? checksApi.update(initialData!.id, check) : checksApi.create(check),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['checks'] })
      onCreated()
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setValidationError(null)

    const error = validatePayload({ name, type, target, sshUser })
    if (error) {
      setValidationError(error)
      return
    }

    mutation.mutate(buildCheckPayload({
      name,
      type,
      server,
      target,
      sshUser,
      enabled,
    }))
  }

  const targetMeta = TARGET_META[type]

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="w-full max-w-lg rounded-xl bg-white p-6 shadow-xl dark:bg-slate-800"
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
              onChange={(e) => setType(e.target.value as CheckConfig['type'])}
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white"
            >
              {CHECK_TYPES.map((t) => (
                <option key={t} value={t}>{checkTypeLabel(t)}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">Server</label>
            <input
              type="text"
              value={server}
              onChange={(e) => setServer(e.target.value)}
              placeholder="server name"
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white dark:placeholder:text-slate-500"
            />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">{targetMeta.label}</label>
            <input
              type="text"
              required
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              placeholder={targetMeta.placeholder}
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-700 dark:text-white dark:placeholder:text-slate-500"
            />
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">{targetMeta.hint}</p>
          </div>

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

          {(validationError || mutation.isError) && (
            <p className="text-sm text-red-600 dark:text-red-400">
              {validationError || (mutation.error instanceof Error ? mutation.error.message : isEditing ? 'Failed to update check' : 'Failed to create check')}
            </p>
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
