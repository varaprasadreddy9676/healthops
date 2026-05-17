import { api } from '@/shared/api/client'

export interface RemediationConfig {
    enabled: boolean
    dryRun: boolean
    maxConcurrent: number
    outputLimitBytes: number
    updatedAt?: string
}

export interface AllowedAction {
    id: string
    name: string
    type: 'command' | 'ssh_command' | 'http'
    command?: string
    url?: string
    method?: string
    headers?: Record<string, string>
    timeoutSeconds: number
    risk: 'low' | 'medium' | 'high'
    description: string
    createdAt: string
    updatedAt: string
}

export interface RemediationAttempt {
    id: string
    checkId: string
    incidentId: string
    actionId: string
    actionName: string
    actionType: 'command' | 'ssh_command' | 'http'
    command: string
    attemptNumber: number
    status: 'pending' | 'running' | 'success' | 'failed' | 'dry_run' | 'timed_out' | 'skipped' | 'escalated'
    dryRun: boolean
    exitCode: number
    output: string
    error?: string
    verified?: boolean
    aiAnalysis?: string
    durationMs: number
    triggeredBy: string
    createdAt: string
}

export const remediationApi = {
    // Config
    getConfig: () =>
        api.get<RemediationConfig>('/remediation/config'),
    saveConfig: (config: RemediationConfig) =>
        api.put<RemediationConfig>('/remediation/config', config),

    // Allowed Actions
    listActions: () =>
        api.get<{ actions: AllowedAction[]; total: number }>('/remediation/actions'),
    getAction: (id: string) =>
        api.get<AllowedAction>(`/remediation/actions/${id}`),
    createAction: (action: Omit<AllowedAction, 'createdAt' | 'updatedAt'>) =>
        api.post<AllowedAction>('/remediation/actions', action),
    updateAction: (id: string, action: Partial<AllowedAction>) =>
        api.put<AllowedAction>(`/remediation/actions/${id}`, action),
    deleteAction: (id: string) =>
        api.delete(`/remediation/actions/${id}`),

    // Attempts
    listAttempts: (params?: { checkId?: string; incidentId?: string; limit?: number; offset?: number }) => {
        const search = new URLSearchParams()
        if (params?.checkId) search.set('checkId', params.checkId)
        if (params?.incidentId) search.set('incidentId', params.incidentId)
        if (params?.limit) search.set('limit', String(params.limit))
        if (params?.offset) search.set('offset', String(params.offset))
        const qs = search.toString()
        return api.get<{ attempts: RemediationAttempt[]; total: number }>(`/remediation/attempts${qs ? `?${qs}` : ''}`)
    },
    getAttempt: (id: string) =>
        api.get<RemediationAttempt>(`/remediation/attempts/${id}`),
    checkAttempts: (checkId: string) =>
        api.get<{ attempts: RemediationAttempt[]; total: number }>(`/checks/${checkId}/remediations`),
    incidentAttempts: (incidentId: string) =>
        api.get<{ attempts: RemediationAttempt[]; total: number }>(`/incidents/${incidentId}/remediations`),

    // Manual trigger
    remediate: (checkId: string, incidentId?: string) =>
        api.post<RemediationAttempt>(`/checks/${checkId}/remediate`, { incidentId, actor: 'manual' }),

    // AI suggest
    suggestCommand: (checkType: string, checkTarget: string, serverHost?: string, failMessage?: string) =>
        api.post<{ command: string }>('/remediation/suggest-command', { checkType, checkTarget, serverHost, failMessage }),
}
