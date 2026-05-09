import { useEffect, useRef, useCallback, useState } from 'react'
import type { SSEPayload } from "@/shared/types"
import { SSE_RECONNECT_DELAY } from "@/shared/lib/constants"

export function useSSE(onMessage: (payload: SSEPayload) => void, enabled = true) {
  const [connected, setConnected] = useState(false)
  const esRef = useRef<EventSource | null>(null)
  const reconnectRef = useRef<number | null>(null)
  const stoppedRef = useRef(false)
  const onMessageRef = useRef(onMessage)
  onMessageRef.current = onMessage

  const connect = useCallback(() => {
    if (!enabled || stoppedRef.current) return
    if (esRef.current) esRef.current.close()
    if (reconnectRef.current) window.clearTimeout(reconnectRef.current)

    const token = localStorage.getItem('healthops_token')
    const url = token ? `/api/v1/events?token=${encodeURIComponent(token)}` : '/api/v1/events'
    const es = new EventSource(url)
    esRef.current = es

    es.onopen = () => setConnected(true)

    // Listen for both named 'snapshot' events and unnamed message events
    const handler = (event: MessageEvent) => {
      try {
        const payload = JSON.parse(event.data) as SSEPayload
        onMessageRef.current(payload)
      } catch { /* ignore malformed events */ }
    }

    es.addEventListener('snapshot', handler as EventListener)
    es.onmessage = handler

    es.onerror = () => {
      setConnected(false)
      es.close()
      if (!stoppedRef.current && enabled) {
        reconnectRef.current = window.setTimeout(connect, SSE_RECONNECT_DELAY)
      }
    }
  }, [enabled])

  useEffect(() => {
    stoppedRef.current = false
    connect()
    return () => {
      stoppedRef.current = true
      if (reconnectRef.current) {
        window.clearTimeout(reconnectRef.current)
        reconnectRef.current = null
      }
      esRef.current?.close()
      esRef.current = null
      setConnected(false)
    }
  }, [connect])

  return { connected }
}
