import { useEffect, useRef, useCallback, useState } from 'react'
import { SSE_RECONNECT_DELAY } from "@/shared/lib/constants"
import type { ServerSnapshot } from "@/shared/types"

export interface ServerLiveState {
  snapshot: ServerSnapshot | null
  history: ServerSnapshot[]
  connected: boolean
}

const MAX_HISTORY = 60

export function useServerLive(serverId: string | undefined, enabled = true): ServerLiveState {
  const [snapshot, setSnapshot] = useState<ServerSnapshot | null>(null)
  const [history, setHistory] = useState<ServerSnapshot[]>([])
  const [connected, setConnected] = useState(false)
  const esRef = useRef<EventSource | null>(null)
  const reconnectRef = useRef<number | null>(null)
  const stoppedRef = useRef(false)

  const connect = useCallback(() => {
    if (!serverId || !enabled || stoppedRef.current) return
    if (esRef.current) esRef.current.close()
    if (reconnectRef.current) window.clearTimeout(reconnectRef.current)

    const token = localStorage.getItem('healthops_token')
    const params = new URLSearchParams()
    params.set('interval', '5')
    if (token) params.set('token', token)
    const url = `/api/v1/servers/${encodeURIComponent(serverId)}/live?${params}`
    const es = new EventSource(url)
    esRef.current = es

    es.onopen = () => setConnected(true)

    es.addEventListener('snapshot', (event) => {
      try {
        const snap = JSON.parse((event as MessageEvent).data) as ServerSnapshot
        setSnapshot(snap)
        setHistory(prev => {
          const next = [...prev, snap]
          return next.length > MAX_HISTORY ? next.slice(-MAX_HISTORY) : next
        })
      } catch { /* ignore malformed */ }
    })

    es.onerror = () => {
      setConnected(false)
      es.close()
      if (!stoppedRef.current && enabled) {
        reconnectRef.current = window.setTimeout(connect, SSE_RECONNECT_DELAY)
      }
    }
  }, [serverId, enabled])

  useEffect(() => {
    stoppedRef.current = false
    if (enabled && serverId) {
      connect()
    }
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
  }, [connect, enabled, serverId])

  return { snapshot: enabled ? snapshot : null, history, connected: enabled && connected }
}
