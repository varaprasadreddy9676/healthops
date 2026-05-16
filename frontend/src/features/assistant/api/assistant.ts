import { api } from '@/shared/api/client'

export interface AssistantMessage {
    role: 'user' | 'assistant'
    content: string
    timestamp: string
}

export interface AssistantReference {
    type: 'check' | 'incident' | 'server' | 'log_family'
    id: string
    name: string
}

export interface AskResponse {
    answer: string
    references?: AssistantReference[]
    durationMs: number
    provider?: string
}

export interface AssistantStatus {
    available: boolean
    model: string
}

export const assistantApi = {
    ask: (question: string, history?: AssistantMessage[], lookbackMinutes?: number) =>
        api.post<AskResponse>('/assistant/ask', { question, history, lookbackMinutes }),
    status: () =>
        api.get<AssistantStatus>('/assistant/status'),
}
