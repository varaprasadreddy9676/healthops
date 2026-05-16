import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, Database } from 'lucide-react'
import { api, APIError } from '@/shared/api/client'
import type { SystemStatus } from '@/shared/types'

export function DegradedBanner() {
  const { data, error } = useQuery({
    queryKey: ['system-status'],
    queryFn: () => api.get<SystemStatus>('/system/status'),
    refetchInterval: 10_000,
    staleTime: 5_000,
    retry: 1,
  })

  if (data?.healthy === false) {
    return (
      <div className="border-b border-red-200 bg-red-50 px-4 py-3 text-red-900 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-100">
        <div className="mx-auto flex max-w-[1600px] items-start gap-3 text-sm sm:px-2 lg:px-4">
          <Database className="mt-0.5 h-4 w-4 flex-none" />
          <div>
            <p className="font-semibold">HealthOps database is unavailable. Writes are blocked until MongoDB recovers.</p>
            <p className="mt-1 text-xs text-red-800 dark:text-red-200">
              {data.lastError || 'MongoDB health check failed.'}
              {data.degradedSince ? ` Degraded since ${new Date(data.degradedSince).toLocaleString()}.` : ''}
            </p>
          </div>
        </div>
      </div>
    )
  }

  if (error instanceof APIError && error.code >= 500) {
    return (
      <div className="border-b border-amber-200 bg-amber-50 px-4 py-3 text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-100">
        <div className="mx-auto flex max-w-[1600px] items-center gap-3 text-sm sm:px-2 lg:px-4">
          <AlertTriangle className="h-4 w-4 flex-none" />
          <p className="font-medium">Unable to verify HealthOps database status. Treat configuration writes as unsafe until status recovers.</p>
        </div>
      </div>
    )
  }

  return null
}
