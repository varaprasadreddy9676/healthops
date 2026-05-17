import { api } from '@/shared/api/client'

export interface LogEntry {
    id: string
    timestamp: string
    level: string
    message: string
    source: string
    server?: string
    category?: string
    stackTrace?: string
    fingerprint: string
    familyId: string
    tags?: string[]
    meta?: Record<string, unknown>
}

export interface ErrorFamily {
    id: string
    fingerprint: string
    title: string
    category: string
    severity: string
    pattern: string
    source: string
    firstSeenAt: string
    lastSeenAt: string
    occurrenceCount: number
    sampleMessages?: string[]
    servers?: string[]
    aiLabel?: string
    aiSummary?: string
    status: string
}

export interface LogFamilyStats {
    totalFamilies: number
    activeFamilies: number
    totalEntries: number
    categoryCounts: Record<string, number>
    severityCounts: Record<string, number>
    topFamilies: ErrorFamily[]
}

export interface LogFamilyDetail {
    family: ErrorFamily
    entries: LogEntry[]
}

function qs(params?: Record<string, string | number | undefined>): string {
    if (!params) return ''
    const parts: string[] = []
    for (const [k, v] of Object.entries(params)) {
        if (v !== undefined && v !== '') parts.push(`${k}=${encodeURIComponent(v)}`)
    }
    return parts.length ? `?${parts.join('&')}` : ''
}

export const logsApi = {
    ingest: (entries: Array<{ level: string; message: string; source: string; server?: string; stackTrace?: string; tags?: string[]; meta?: Record<string, unknown> }>) =>
        api.post<{ ingested: number; families: number }>('/logs/ingest', { entries }),

    entries: (params?: { source?: string; limit?: number }) =>
        api.get<LogEntry[]>(`/logs/entries${qs(params)}`),

    families: (params?: { status?: string; category?: string; limit?: number }) =>
        api.get<ErrorFamily[]>(`/logs/families${qs(params)}`),

    familyDetail: (id: string) =>
        api.get<LogFamilyDetail>(`/logs/families/${encodeURIComponent(id)}`),

    updateFamily: (id: string, data: { status?: string; category?: string; severity?: string }) =>
        api.patch<ErrorFamily>(`/logs/families/${encodeURIComponent(id)}`, data),

    categorizeFamily: (id: string) =>
        api.post<ErrorFamily>(`/logs/families/${encodeURIComponent(id)}/categorize`),

    stats: () =>
        api.get<LogFamilyStats>('/logs/stats'),

    categorize: (limit?: number) =>
        api.post<{ categorized: number }>(`/logs/categorize${limit ? `?limit=${limit}` : ''}`),
}
