import { api } from '@/shared/api/client'
import type { MySQLHealthCard } from "@/shared/types"

export interface MySQLAIResponse {
  answer: string
  suggestions?: string[]
  severity?: 'info' | 'warning' | 'critical'
  provider: string
  model: string
  answeredAt: string
}

export interface KillQueryResponse {
  processId: number
  status: string
}

export const mysqlApi = {
  health: (checkId?: string) => {
    const qs = checkId ? `?checkId=${encodeURIComponent(checkId)}` : ''
    return api.get<MySQLHealthCard>(`/mysql/health${qs}`)
  },
  samples: (params?: { limit?: number; checkId?: string }) => {
    const qs = new URLSearchParams()
    if (params?.limit) qs.set('limit', String(params.limit))
    if (params?.checkId) qs.set('checkId', params.checkId)
    const q = qs.toString()
    return api.get<unknown[]>(`/mysql/samples${q ? `?${q}` : ''}`)
  },
  deltas: (params?: { limit?: number; checkId?: string }) => {
    const qs = new URLSearchParams()
    if (params?.limit) qs.set('limit', String(params.limit))
    if (params?.checkId) qs.set('checkId', params.checkId)
    const q = qs.toString()
    return api.get<unknown[]>(`/mysql/deltas${q ? `?${q}` : ''}`)
  },
  timeseries: (params?: { metric?: string; period?: string; checkId?: string }) => {
    const qs = new URLSearchParams()
    if (params?.metric) qs.set('metric', params.metric)
    if (params?.period) qs.set('period', params.period)
    if (params?.checkId) qs.set('checkId', params.checkId)
    const q = qs.toString()
    return api.get<unknown[]>(`/mysql/timeseries${q ? `?${q}` : ''}`)
  },
  killQuery: (processId: number, checkId?: string) =>
    api.post<KillQueryResponse>('/mysql/kill', { processId, checkId: checkId || '' }),
  aiAsk: (question?: string, providerId?: string, checkId?: string) =>
    api.post<MySQLAIResponse>('/mysql/ai/ask', { question: question || '', providerId: providerId || '', checkId: checkId || '' }),
}
