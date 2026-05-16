import { api } from '@/shared/api/client'

export interface AutomationAction {
    id: string
    type: 'restart' | 'drain_node' | 'rotate_credential' | 'inspect_queries' | 'scale_up' | 'clear_queue' | 'custom'
    title: string
    description: string
    risk: 'low' | 'medium' | 'high' | 'critical'
    checkId?: string
    server?: string
    incidentId?: string
    command?: string
    params?: Record<string, unknown>
    reason: string
    status: 'pending' | 'approved' | 'rejected' | 'expired'
    createdAt: string
    expiresAt: string
    approvedBy?: string
    approvedAt?: string
    rejectedBy?: string
    rejectedAt?: string
    executedAt?: string
    result?: string
}

export interface AuditEntry {
    id: string
    actionId: string
    actor: string
    event: string
    details?: string
    timestamp: string
}

export interface AutomationStatus {
    enabled: boolean
    aiAvailable: boolean
}

export const automationApi = {
    listActions: (status?: string) => {
        const params = status ? `?status=${status}` : ''
        return api.get<{ actions: AutomationAction[]; total: number }>(`/automation/actions${params}`)
    },
    getAction: (id: string) =>
        api.get<AutomationAction>(`/automation/actions/${id}`),
    suggest: (incidentId?: string, checkId?: string, context?: string) =>
        api.post<{ actions: AutomationAction[]; generatedAt: string }>('/automation/suggest', { incidentId, checkId, context }),
    approve: (id: string, actor: string) =>
        api.post<{ id: string; status: string }>(`/automation/actions/${id}/approve`, { actor }),
    reject: (id: string, actor: string, reason?: string) =>
        api.post<{ id: string; status: string }>(`/automation/actions/${id}/reject`, { actor, reason }),
    audit: () =>
        api.get<{ entries: AuditEntry[]; total: number }>('/automation/audit'),
    status: () =>
        api.get<AutomationStatus>('/automation/status'),
}
