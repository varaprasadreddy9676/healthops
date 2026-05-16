import { api } from '@/shared/api/client'

export interface RCAHypothesis {
    rank: number
    title: string
    description: string
    confidence: number
    category: string
    evidence: string[]
    suggestion: string
}

export interface TimelineEvent {
    timestamp: string
    type: string
    description: string
    severity?: string
    source?: string
}

export interface SignalSeries {
    name: string
    source: string
    server?: string
    trend: string
    min: number
    max: number
    avg: number
    points: { timestamp: string; value: number }[]
}

export interface RCAReport {
    id: string
    incidentId: string
    createdAt: string
    status: string
    hypotheses?: RCAHypothesis[]
    summary?: string
    timeline?: TimelineEvent[]
    signalCount: number
    windowStart: string
    windowEnd: string
    providerUsed?: string
    error?: string
}

export interface RCATimeline {
    timeline: TimelineEvent[]
    signals: SignalSeries[]
}

export const rcaApi = {
    analyze: (incidentId: string) =>
        api.post<RCAReport>(`/rca/analyze/${encodeURIComponent(incidentId)}`),
    reports: (incidentId: string) =>
        api.get<RCAReport[]>(`/rca/reports/${encodeURIComponent(incidentId)}`),
    report: (id: string) =>
        api.get<RCAReport>(`/rca/report/${encodeURIComponent(id)}`),
    timeline: (incidentId: string) =>
        api.get<RCATimeline>(`/rca/timeline/${encodeURIComponent(incidentId)}`),
    allReports: (limit?: number) => {
        const qs = limit ? `?limit=${limit}` : ''
        return api.get<RCAReport[]>(`/rca/reports${qs}`)
    },
}
