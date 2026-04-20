import { api } from './client'
import type { RemoteServer, ServerTestResult, ServerSnapshot, MetricsPoint, ProcessInfo } from '@/types'

export const serversApi = {
  list: () => api.get<RemoteServer[]>('/servers'),
  get: (id: string) => api.get<RemoteServer>(`/servers/${encodeURIComponent(id)}`),
  create: (server: Partial<RemoteServer>) => api.post<RemoteServer>('/servers', server),
  update: (id: string, server: Partial<RemoteServer>) => api.put<RemoteServer>(`/servers/${encodeURIComponent(id)}`, server),
  delete: (id: string) => api.delete(`/servers/${encodeURIComponent(id)}`),
  test: (id: string) => api.post<ServerTestResult>(`/servers/${encodeURIComponent(id)}/test`),
  metrics: (id: string) => api.get<ServerSnapshot>(`/servers/${encodeURIComponent(id)}/metrics`),
  processes: (id: string) => api.get<ProcessInfo[]>(`/servers/${encodeURIComponent(id)}/processes`),
  metricsHistory: (id: string, range = '24h') =>
    api.get<MetricsPoint[]>(`/servers/${encodeURIComponent(id)}/metrics/history?range=${range}`),
}
