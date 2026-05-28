import { api } from '@/shared/api/client'

// --- Types matching backend evidence/types.go ---

export interface ConfidenceBreakdown {
    hasDeploymentCorrelation: boolean
    hasMetricAnomaly: boolean
    hasLogFingerprintSpike: boolean
    hasSimilarPastIncident: boolean
    evidenceCount: number
    evidenceCountNormalized: number
    availableCategories: number
    totalPossibleCategories: number
}

export interface ConfidenceScore {
    score: number
    band: 'low' | 'medium' | 'high'
    breakdown: ConfidenceBreakdown
}

export interface EvidenceCitation {
    category: string
    description: string
    signalId?: string
    timestamp?: string
}

export interface EvidenceLedgerItem {
    id: string
    claim: string
    status: 'supported' | 'unsupported' | 'contradicted' | 'missing'
    category: string
    confidenceImpact: 'positive' | 'negative' | 'neutral'
    evidenceIds?: string[]
    rationale: string
    attributes?: Record<string, string>
}

export interface EvidenceLedgerSummary {
    supported: number
    unsupported: number
    contradicted: number
    missing: number
}

export interface BriefTimelineEntry {
    time: string
    description: string
}

export interface BriefMetadata {
    availableCategories: string[]
    missingCategories: string[]
    evidenceCount: number
    evidenceCap: number
    wasCapped: boolean
    durationMs: number
}

export interface IncidentBrief {
    incidentId: string
    generatedAt: string
    likelyCause: string
    confidence: ConfidenceScore
    evidenceSummary: EvidenceCitation[]
    evidenceLedger?: EvidenceLedgerItem[]
    evidenceLedgerSummary?: EvidenceLedgerSummary
    nextActions: string[]
    impactSummary?: string
    timeline?: BriefTimelineEntry[]
    metadata: BriefMetadata
    rawAiResponse?: string
}

export interface IncidentTimelineEvent {
    id: string
    incidentId: string
    type: string
    timestamp: string
    source: string
    message: string
    signalIds?: string[]
    attributes?: Record<string, string>
}

export interface TimelineResponse {
    incidentId: string
    events: IncidentTimelineEvent[]
    count: number
}

export const evidenceApi = {
    getTimeline: (incidentId: string) =>
        api.get<TimelineResponse>(`/evidence/incidents/${encodeURIComponent(incidentId)}/timeline`),

    getBrief: (incidentId: string) =>
        api.get<IncidentBrief>(`/evidence/incidents/${encodeURIComponent(incidentId)}/brief`),

    generateBrief: (incidentId: string) =>
        api.post<IncidentBrief>(`/evidence/incidents/${encodeURIComponent(incidentId)}/brief`),
}
