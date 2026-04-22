import { api } from '@/shared/api/client'
import type { CheckConfig, CheckDetail, CheckResult } from "@/shared/types"

export const checksApi = {
  list: () => api.get<CheckConfig[]>('/checks'),
  get: (id: string) => api.get<CheckDetail>(`/checks/${encodeURIComponent(id)}`),
  create: (check: Partial<CheckConfig>) => api.post<CheckConfig>('/checks', check),
  update: (id: string, check: Partial<CheckConfig>) => api.put<CheckConfig>(`/checks/${encodeURIComponent(id)}`, check),
  delete: (id: string) => api.delete(`/checks/${encodeURIComponent(id)}`),
  results: (checkId?: string) => api.get<CheckResult[]>(`/results${checkId ? `?checkId=${encodeURIComponent(checkId)}` : ''}`),
}
