import { useState, useRef, useEffect, useCallback } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { Send, Bot, User, Loader2, AlertCircle, ExternalLink, Sparkles } from 'lucide-react'
import { cn, relativeTime } from '@/shared/lib/utils'
import { assistantApi, type AssistantMessage, type AssistantReference, type AskResponse } from '@/features/assistant/api/assistant'
import { Link } from 'react-router-dom'
import { MarkdownContent } from '@/shared/components/MarkdownContent'

interface ChatMessage {
    role: 'user' | 'assistant'
    content: string
    timestamp: string
    references?: AssistantReference[]
    durationMs?: number
}

const SUGGESTIONS = [
    'Why is prod slow right now?',
    'Show unhealthy checks',
    'Any incidents in the last hour?',
    'Which server has the most failures?',
    'Summarize current system health',
]

const MAX_VISIBLE_REFERENCES = 8

function uniqueReferences(refs: AssistantReference[] = []): AssistantReference[] {
    const seen = new Set<string>()
    return refs.filter((ref) => {
        const key = `${ref.type}:${ref.id}`
        if (seen.has(key)) return false
        seen.add(key)
        return true
    })
}

function ReferenceChip({ ref }: { ref: AssistantReference }) {
    const linkMap: Record<string, string> = {
        check: `/checks/${ref.id}`,
        incident: `/incidents/${ref.id}`,
        server: `/servers/${ref.id}`,
        log_family: `/logs/${ref.id}`,
    }
    const href = linkMap[ref.type] || '#'

    return (
        <Link
            to={href}
            className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-slate-50 px-2 py-0.5 text-[11px] font-medium text-slate-600 transition-colors hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400 dark:hover:border-blue-700 dark:hover:text-blue-400"
        >
            <span className="capitalize">{ref.type.replace('_', ' ')}</span>
            <span className="text-slate-400">·</span>
            <span className="truncate max-w-[120px]">{ref.name}</span>
            <ExternalLink className="h-2.5 w-2.5 shrink-0" />
        </Link>
    )
}

function MessageBubble({ message }: { message: ChatMessage }) {
    const isUser = message.role === 'user'
    const references = uniqueReferences(message.references)
    const visibleReferences = references.slice(0, MAX_VISIBLE_REFERENCES)
    const hiddenReferenceCount = references.length - visibleReferences.length

    return (
        <div className={cn('flex gap-3', isUser ? 'flex-row-reverse' : 'flex-row')}>
            <div className={cn(
                'flex h-7 w-7 shrink-0 items-center justify-center rounded-full',
                isUser ? 'bg-blue-100 dark:bg-blue-900/40' : 'bg-violet-100 dark:bg-violet-900/40'
            )}>
                {isUser ? (
                    <User className="h-3.5 w-3.5 text-blue-600 dark:text-blue-400" />
                ) : (
                    <Bot className="h-3.5 w-3.5 text-violet-600 dark:text-violet-400" />
                )}
            </div>
            <div className={cn('max-w-[80%] space-y-2', isUser ? 'items-end' : 'items-start')}>
                <div className={cn(
                    'rounded-xl px-4 py-2.5 text-sm',
                    isUser
                        ? 'bg-blue-600 text-white dark:bg-blue-700'
                        : 'bg-white border border-slate-200 text-slate-700 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300'
                )}>
                    {isUser ? (
                        <p>{message.content}</p>
                    ) : (
                        <MarkdownContent text={message.content} className="text-sm" />
                    )}
                </div>
                {visibleReferences.length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                        {visibleReferences.map((ref, i) => (
                            <ReferenceChip key={`${ref.type}-${ref.id}-${i}`} ref={ref} />
                        ))}
                        {hiddenReferenceCount > 0 && (
                            <span className="inline-flex items-center rounded-md border border-slate-200 bg-white px-2 py-0.5 text-[11px] font-medium text-slate-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400">
                                +{hiddenReferenceCount} more in context
                            </span>
                        )}
                    </div>
                )}
                <div className="flex items-center gap-2 text-[10px] text-slate-400">
                    <span>{relativeTime(message.timestamp)}</span>
                    {message.durationMs != null && message.durationMs > 0 && (
                        <span>· {message.durationMs}ms</span>
                    )}
                </div>
            </div>
        </div>
    )
}

export default function Assistant() {
    const [messages, setMessages] = useState<ChatMessage[]>([])
    const [input, setInput] = useState('')
    const [lookbackMinutes, setLookbackMinutes] = useState(2880) // 48h default
    const scrollRef = useRef<HTMLDivElement>(null)
    const inputRef = useRef<HTMLInputElement>(null)

    const { data: status } = useQuery({
        queryKey: ['assistant', 'status'],
        queryFn: assistantApi.status,
    })

    const askMutation = useMutation({
        mutationFn: (question: string) => {
            const history: AssistantMessage[] = messages.slice(-10).map((m) => ({
                role: m.role,
                content: m.content,
                timestamp: m.timestamp,
            }))
            return assistantApi.ask(question, history, lookbackMinutes)
        },
        onSuccess: (data: AskResponse) => {
            setMessages((prev) => [...prev, {
                role: 'assistant',
                content: data.answer,
                timestamp: new Date().toISOString(),
                references: data.references,
                durationMs: data.durationMs,
            }])
        },
        onError: (error: Error) => {
            setMessages((prev) => [...prev, {
                role: 'assistant',
                content: `Sorry, I encountered an error: ${error.message}`,
                timestamp: new Date().toISOString(),
            }])
        },
    })

    const handleSubmit = useCallback((question: string) => {
        if (!question.trim() || askMutation.isPending) return

        setMessages((prev) => [...prev, {
            role: 'user',
            content: question.trim(),
            timestamp: new Date().toISOString(),
        }])
        setInput('')
        askMutation.mutate(question.trim())
    }, [askMutation])

    useEffect(() => {
        scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' })
    }, [messages, askMutation.isPending])

    useEffect(() => {
        inputRef.current?.focus()
    }, [])

    const isAvailable = status?.available ?? false

    return (
        <div className="flex h-[calc(100vh-4rem)] flex-col animate-fade-in">
            {/* Header */}
            <div className="flex items-center gap-3 border-b border-slate-200 px-5 py-3 dark:border-slate-700">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-600">
                    <Sparkles className="h-4 w-4 text-white" />
                </div>
                <div>
                    <h1 className="text-base font-semibold text-slate-900 dark:text-slate-100">Ops Assistant</h1>
                    <p className="text-xs text-slate-500">
                        {isAvailable ? 'Ask questions about your infrastructure' : 'AI provider not configured'}
                    </p>
                </div>
                {isAvailable && (
                    <span className="ml-auto inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
                        <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
                        Online
                    </span>
                )}
            </div>

            {/* Messages area */}
            <div ref={scrollRef} className="flex-1 overflow-y-auto px-5 py-4 space-y-4">
                {messages.length === 0 && (
                    <div className="flex h-full flex-col items-center justify-center gap-4 text-center">
                        <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-violet-100 to-purple-100 dark:from-violet-900/30 dark:to-purple-900/30">
                            <Bot className="h-7 w-7 text-violet-600 dark:text-violet-400" />
                        </div>
                        <div>
                            <h2 className="text-sm font-semibold text-slate-700 dark:text-slate-300">HealthOps Assistant</h2>
                            <p className="mt-1 text-xs text-slate-500 max-w-sm">
                                Ask questions about your infrastructure health. I answer from real telemetry data — checks, incidents, servers, and logs.
                            </p>
                        </div>
                        {isAvailable && (
                            <div className="flex flex-wrap justify-center gap-2 mt-2">
                                {SUGGESTIONS.map((s) => (
                                    <button
                                        key={s}
                                        onClick={() => handleSubmit(s)}
                                        className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-600 transition-colors hover:border-violet-300 hover:bg-violet-50 hover:text-violet-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400 dark:hover:border-violet-700 dark:hover:text-violet-400"
                                    >
                                        {s}
                                    </button>
                                ))}
                            </div>
                        )}
                    </div>
                )}
                {messages.map((msg, i) => (
                    <MessageBubble key={i} message={msg} />
                ))}
                {askMutation.isPending && (
                    <div className="flex gap-3">
                        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-violet-100 dark:bg-violet-900/40">
                            <Bot className="h-3.5 w-3.5 text-violet-600 dark:text-violet-400" />
                        </div>
                        <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-2.5 dark:border-slate-700 dark:bg-slate-800">
                            <Loader2 className="h-3.5 w-3.5 animate-spin text-violet-500" />
                            <span className="text-xs text-slate-500">Analyzing telemetry...</span>
                        </div>
                    </div>
                )}
            </div>

            {/* Input area */}
            <div className="border-t border-slate-200 px-5 py-3 dark:border-slate-700">
                {!isAvailable ? (
                    <div className="flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2.5 dark:border-amber-900 dark:bg-amber-950/30">
                        <AlertCircle className="h-4 w-4 shrink-0 text-amber-500" />
                        <p className="text-xs text-amber-700 dark:text-amber-400">
                            No AI provider configured. Go to <Link to="/settings" className="underline font-medium">Settings</Link> to add an API key.
                        </p>
                    </div>
                ) : (
                    <div className="space-y-2">
                        <form
                            onSubmit={(e) => { e.preventDefault(); handleSubmit(input) }}
                            className="flex items-center gap-2"
                        >
                            <input
                                ref={inputRef}
                                type="text"
                                value={input}
                                onChange={(e) => setInput(e.target.value)}
                                placeholder="Ask about your infrastructure..."
                                disabled={askMutation.isPending}
                                className="flex-1 rounded-lg border border-slate-200 bg-white px-4 py-2.5 text-sm text-slate-700 placeholder-slate-400 outline-none transition-colors focus:border-violet-400 focus:ring-2 focus:ring-violet-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:focus:border-violet-600 dark:focus:ring-violet-900/30"
                                maxLength={2000}
                            />
                            <select
                                value={lookbackMinutes}
                                onChange={(e) => setLookbackMinutes(Number(e.target.value))}
                                className="h-10 rounded-lg border border-slate-200 bg-white px-2 text-xs text-slate-600 outline-none transition-colors focus:border-violet-400 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400"
                                title="Lookback window"
                            >
                                <option value={60}>1h</option>
                                <option value={360}>6h</option>
                                <option value={1440}>24h</option>
                                <option value={2880}>48h</option>
                                <option value={10080}>7d</option>
                            </select>
                            <button
                                type="submit"
                                disabled={!input.trim() || askMutation.isPending}
                                className="flex h-10 w-10 items-center justify-center rounded-lg bg-violet-600 text-white transition-colors hover:bg-violet-700 disabled:opacity-40 disabled:cursor-not-allowed"
                            >
                                <Send className="h-4 w-4" />
                            </button>
                        </form>
                    </div>
                )}
            </div>
        </div>
    )
}
