import { api } from '@/shared/api/client'
import type { DashboardSnapshot, OverviewStats, Summary, RunSummary } from "@/shared/types"

export const dashboardApi = {
  snapshot: () => api.get<DashboardSnapshot>('/dashboard'),
  summary: () => api.get<Summary>('/summary'),
  overview: () => api.get<OverviewStats>('/stats/overview'),
  runNow: () => api.post<RunSummary>('/runs'),
}
