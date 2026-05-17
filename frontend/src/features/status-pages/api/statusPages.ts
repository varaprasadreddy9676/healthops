import { api } from '@/shared/api/client'
import type { StatusPageConfig } from '@/shared/types'

export const statusPagesApi = {
  list: () => api.get<StatusPageConfig[]>('/status-pages'),
  create: (page: Partial<StatusPageConfig>) => api.post<StatusPageConfig>('/status-pages', page),
  update: (id: string, page: Partial<StatusPageConfig>) =>
    api.put<StatusPageConfig>(`/status-pages/${encodeURIComponent(id)}`, page),
  delete: (id: string) => api.delete(`/status-pages/${encodeURIComponent(id)}`),
}
