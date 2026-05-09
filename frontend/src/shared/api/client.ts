import { API_BASE } from "@/shared/lib/constants"
import type { APIResponse } from "@/shared/types"

class APIError extends Error {
  constructor(public code: number, message: string) {
    super(message)
    this.name = 'APIError'
  }
}

let isRedirecting = false
const REQUEST_TIMEOUT_MS = 30_000

function clearAuthState() {
  localStorage.removeItem('healthops_token')
  localStorage.removeItem('healthops_user')
}

function parseFilename(contentDisposition: string | null, fallback: string) {
  if (!contentDisposition) return fallback
  const match = contentDisposition.match(/filename="?([^";]+)"?/i)
  return match?.[1] || fallback
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const url = `${API_BASE}${path}`
  const token = localStorage.getItem('healthops_token')

  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)

  let res: Response
  try {
    res = await fetch(url, {
      ...options,
      signal: controller.signal,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options?.headers,
      },
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new APIError(408, 'Request timed out')
    }
    throw err
  } finally {
    clearTimeout(timeoutId)
  }

  if (res.status === 401 && !isRedirecting) {
    isRedirecting = true
    clearAuthState()
    window.location.href = '/login'
    throw new APIError(401, 'Session expired')
  }

  const raw = res.status === 204 ? '' : await res.text()

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = raw ? JSON.parse(raw) as APIResponse : undefined
      if (body?.error?.message) message = body.error.message
    } catch { /* ignore parse errors */ }
    throw new APIError(res.status, message)
  }

  if (!raw) {
    return undefined as T
  }

  const body = JSON.parse(raw) as APIResponse<T>
  if (!body.success && body.error) {
    throw new APIError(body.error.code, body.error.message)
  }

  return body.data as T
}

async function download(path: string, fallbackFilename: string) {
  const token = localStorage.getItem('healthops_token')
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)

  let res: Response
  try {
    res = await fetch(path, {
      signal: controller.signal,
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new APIError(408, 'Export timed out')
    }
    throw err
  } finally {
    clearTimeout(timeoutId)
  }

  if (res.status === 401 && !isRedirecting) {
    isRedirecting = true
    clearAuthState()
    window.location.href = '/login'
    throw new APIError(401, 'Session expired')
  }

  if (!res.ok) {
    const raw = await res.text().catch(() => '')
    let message = res.statusText
    try {
      const body = raw ? JSON.parse(raw) as APIResponse : undefined
      if (body?.error?.message) message = body.error.message
    } catch { /* ignore parse errors */ }
    throw new APIError(res.status, message)
  }

  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = parseFilename(res.headers.get('Content-Disposition'), fallbackFilename)
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export const api = {
  get: <T>(path: string) => request<T>(path),

  post: <T>(path: string, data?: unknown) =>
    request<T>(path, { method: 'POST', body: data === undefined ? undefined : JSON.stringify(data) }),

  put: <T>(path: string, data?: unknown) =>
    request<T>(path, { method: 'PUT', body: data === undefined ? undefined : JSON.stringify(data) }),

  delete: <T>(path: string) =>
    request<T>(path, { method: 'DELETE' }),

  download,
}

export { APIError }
