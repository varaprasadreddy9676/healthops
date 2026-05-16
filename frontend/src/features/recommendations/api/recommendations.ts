import { api } from '@/shared/api/client'

export interface Recommendation {
    id: string
    category: 'threshold' | 'coverage' | 'stuck'
    priority: 'high' | 'medium' | 'low'
    title: string
    description: string
    checkId?: string
    server?: string
    current?: Record<string, unknown>
    suggested?: Record<string, unknown>
    reason: string
    createdAt: string
    dismissed: boolean
    dismissedAt?: string
}

export interface RecommendationsResponse {
    recommendations: Recommendation[]
    total: number
    generatedAt: string
}

export interface GenerateResponse {
    recommendations: Recommendation[]
    generatedAt: string
    aiEnriched: boolean
}

export const recommendationsApi = {
    list: (category?: string) => {
        const params = category ? `?category=${category}` : ''
        return api.get<RecommendationsResponse>(`/recommendations${params}`)
    },
    generate: (useAi = false) =>
        api.post<GenerateResponse>('/recommendations/generate', { useAi }),
    dismiss: (id: string, reason?: string) =>
        api.post<{ id: string; status: string }>(`/recommendations/${id}/dismiss`, { reason }),
}
