import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ExternalLink, Globe2, Plus, Trash2 } from 'lucide-react'
import { statusPagesApi } from '@/features/status-pages/api/statusPages'
import { checksApi } from '@/features/checks/api/checks'
import { LoadingState } from '@/shared/components/LoadingState'
import { ErrorState } from '@/shared/components/ErrorState'
import { EmptyState } from '@/shared/components/EmptyState'
import { useToast } from '@/shared/components/Toast'
import { checkTypeLabel, cn } from '@/shared/lib/utils'
import type { CheckConfig, StatusPageConfig } from '@/shared/types'

function slugify(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function selectedChecksSummary(checks: CheckConfig[], selected: Set<string>): string {
  if (selected.size === 0) return 'No checks selected'
  const names = checks.filter((c) => selected.has(c.id)).map((c) => c.name)
  if (names.length <= 2) return names.join(', ')
  return `${names.slice(0, 2).join(', ')} +${names.length - 2} more`
}

export default function StatusPages() {
  const queryClient = useQueryClient()
  const toast = useToast()
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [componentName, setComponentName] = useState('Core services')
  const [selectedCheckIds, setSelectedCheckIds] = useState<Set<string>>(new Set())

  const { data: pages, isLoading, error, refetch } = useQuery({
    queryKey: ['status-pages'],
    queryFn: statusPagesApi.list,
  })

  const { data: checks = [] } = useQuery({
    queryKey: ['checks'],
    queryFn: checksApi.list,
  })

  const enabledChecks = useMemo(() => checks.filter((c) => c.enabled !== false), [checks])
  const formSlug = slug || slugify(name)

  const createMutation = useMutation({
    mutationFn: () => {
      const checkIds = Array.from(selectedCheckIds)
      return statusPagesApi.create({
        name: name.trim(),
        slug: formSlug,
        description: description.trim() || undefined,
        isPublic: true,
        showIncidents: true,
        showUptime: true,
        components: [{
          id: slugify(componentName || 'component') || 'component',
          name: componentName.trim() || 'Core services',
          checkIds,
          order: 0,
        }],
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['status-pages'] })
      setName('')
      setSlug('')
      setDescription('')
      setComponentName('Core services')
      setSelectedCheckIds(new Set())
      toast.success('Status page created')
    },
    onError: (err: Error) => toast.error(err.message || 'Failed to create status page'),
  })

  const updateMutation = useMutation({
    mutationFn: ({ page, patch }: { page: StatusPageConfig; patch: Partial<StatusPageConfig> }) =>
      statusPagesApi.update(page.id, patch),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['status-pages'] })
      toast.success('Status page updated')
    },
    onError: (err: Error) => toast.error(err.message || 'Failed to update status page'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => statusPagesApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['status-pages'] })
      toast.success('Status page deleted')
    },
    onError: (err: Error) => toast.error(err.message || 'Failed to delete status page'),
  })

  const toggleCheck = (id: string) => {
    setSelectedCheckIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      toast.error('Name is required')
      return
    }
    if (!formSlug) {
      toast.error('Slug is required')
      return
    }
    if (selectedCheckIds.size === 0) {
      toast.error('Select at least one check')
      return
    }
    createMutation.mutate()
  }

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState message={error.message} retry={() => refetch()} />

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">Status Pages</h1>
          <p className="text-sm text-slate-500">Create public health pages from monitored checks.</p>
        </div>
      </div>

      <form onSubmit={handleCreate} className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
        <div className="mb-4 flex items-center gap-2">
          <Globe2 className="h-4 w-4 text-blue-600" />
          <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">New status page</h2>
        </div>
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="space-y-3">
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Name</label>
              <input
                value={name}
                onChange={(e) => {
                  setName(e.target.value)
                  if (!slug) setSlug(slugify(e.target.value))
                }}
                placeholder="Acme Status"
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Slug</label>
              <input
                value={formSlug}
                onChange={(e) => setSlug(slugify(e.target.value))}
                placeholder="acme-status"
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-mono dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Description</label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={2}
                placeholder="Current status for production systems"
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Component name</label>
              <input
                value={componentName}
                onChange={(e) => setComponentName(e.target.value)}
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              />
            </div>
          </div>

          <div>
            <div className="mb-1 flex items-center justify-between">
              <label className="text-xs font-medium text-slate-600 dark:text-slate-400">Checks</label>
              <span className="text-xs text-slate-400">{selectedChecksSummary(enabledChecks, selectedCheckIds)}</span>
            </div>
            <div className="max-h-72 overflow-y-auto rounded-lg border border-slate-200 dark:border-slate-700">
              {enabledChecks.map((check) => (
                <label
                  key={check.id}
                  className="flex cursor-pointer items-start gap-3 border-b border-slate-100 px-3 py-2 last:border-b-0 hover:bg-slate-50 dark:border-slate-800 dark:hover:bg-slate-800/60"
                >
                  <input
                    type="checkbox"
                    checked={selectedCheckIds.has(check.id)}
                    onChange={() => toggleCheck(check.id)}
                    className="mt-1"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-slate-800 dark:text-slate-200">{check.name}</span>
                    <span className="text-xs text-slate-500">
                      {checkTypeLabel(check.type)}{check.server ? ` · ${check.server}` : ''}
                    </span>
                  </span>
                </label>
              ))}
              {enabledChecks.length === 0 && (
                <div className="p-4 text-sm text-slate-500">No enabled checks are available.</div>
              )}
            </div>
          </div>
        </div>
        <div className="mt-4 flex justify-end">
          <button
            type="submit"
            disabled={createMutation.isPending}
            className="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            <Plus className="h-4 w-4" />
            {createMutation.isPending ? 'Creating...' : 'Create Page'}
          </button>
        </div>
      </form>

      {!pages || pages.length === 0 ? (
        <EmptyState title="No status pages" description="Create a public page for customers or internal stakeholders." />
      ) : (
        <div className="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-100 bg-slate-50/50 dark:border-slate-800 dark:bg-slate-800/30">
                <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">Page</th>
                <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">Visibility</th>
                <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">Components</th>
                <th className="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wider text-slate-500">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {pages.map((page) => (
                <tr key={page.id} className="hover:bg-slate-50 dark:hover:bg-slate-800/50">
                  <td className="px-4 py-3">
                    <div className="font-medium text-slate-900 dark:text-slate-100">{page.name}</div>
                    <div className="font-mono text-xs text-slate-400">/status/{page.slug}</div>
                  </td>
                  <td className="px-4 py-3">
                    <button
                      type="button"
                      onClick={() => updateMutation.mutate({ page, patch: { isPublic: !page.isPublic } })}
                      className={cn(
                        'rounded-full px-2.5 py-1 text-xs font-medium',
                        page.isPublic
                          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                          : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400',
                      )}
                    >
                      {page.isPublic ? 'Public' : 'Private'}
                    </button>
                  </td>
                  <td className="px-4 py-3 text-slate-500">{page.components?.length ?? 0}</td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-2">
                      <a
                        href={`/status/${page.slug}`}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950/40"
                      >
                        <ExternalLink className="h-3.5 w-3.5" />
                        Open
                      </a>
                      <button
                        type="button"
                        onClick={() => deleteMutation.mutate(page.id)}
                        className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/40"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
