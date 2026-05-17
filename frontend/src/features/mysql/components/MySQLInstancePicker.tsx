import { useSearchParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Database, Plus } from 'lucide-react'
import { checksApi } from '@/features/checks/api/checks'
import type { CheckConfig } from '@/shared/types'

/**
 * Returns the currently selected MySQL check id (from ?checkId= URL param),
 * the list of all MySQL checks, and a setter that updates the URL.
 *
 * When no ?checkId= param is set, returns undefined — backend will fall back
 * to the first MySQL check, preserving legacy behavior. `effective` is the
 * resolved check (selected or first available) so callers can show its name/host.
 */
export function useMySQLCheckSelection() {
    const [searchParams, setSearchParams] = useSearchParams()
    const selected = searchParams.get('checkId') || undefined

    const { data: checks = [] } = useQuery({
        queryKey: ['checks'],
        queryFn: () => checksApi.list(),
        staleTime: 30_000,
    })

    const mysqlChecks = checks.filter((c: CheckConfig) => c.type === 'mysql' && c.enabled)

    const setSelected = (id: string | undefined) => {
        const next = new URLSearchParams(searchParams)
        if (id) next.set('checkId', id)
        else next.delete('checkId')
        setSearchParams(next, { replace: true })
    }

    const effective = mysqlChecks.find(c => c.id === selected) ?? mysqlChecks[0]

    return { selected, mysqlChecks, setSelected, effective }
}

function describeTarget(c: CheckConfig): string {
    if (c.host) return c.port ? `${c.host}:${c.port}` : c.host
    if (c.server) return c.server
    return ''
}

interface Props {
    selected?: string
    options: CheckConfig[]
    onChange: (id: string | undefined) => void
}

export function MySQLInstancePicker({ selected, options, onChange }: Props) {
    const current = options.find(c => c.id === selected) ?? options[0]
    const target = current ? describeTarget(current) : ''

    return (
        <div className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-2 py-1 dark:border-slate-700 dark:bg-slate-800/60">
            <Database className="h-4 w-4 text-blue-500" />
            <label htmlFor="mysql-instance" className="sr-only">MySQL instance</label>
            {options.length > 0 ? (
                <>
                    <select
                        id="mysql-instance"
                        value={selected ?? (current ? current.id : '')}
                        onChange={(e) => onChange(e.target.value || undefined)}
                        className="appearance-none rounded bg-transparent px-1 py-0.5 text-sm font-semibold text-slate-900 focus:outline-none dark:text-slate-100"
                    >
                        {options.map((c) => (
                            <option key={c.id} value={c.id}>
                                {c.name}
                            </option>
                        ))}
                    </select>
                    {target && (
                        <span className="font-mono text-xs text-slate-500 dark:text-slate-400">
                            {target}
                        </span>
                    )}
                </>
            ) : (
                <span className="text-sm text-slate-500 dark:text-slate-400">No MySQL servers</span>
            )}
            <Link
                to="/checks?add=mysql"
                title="Add a MySQL server"
                aria-label="Add MySQL server"
                className="ml-1 inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-blue-600 dark:hover:bg-slate-700"
            >
                <Plus className="h-3.5 w-3.5" />
            </Link>
        </div>
    )
}
