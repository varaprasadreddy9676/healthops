import { api } from '@/shared/api/client'

// Topic returned from the backend `/api/v1/help/topics/{slug}` endpoint.
export interface HelpTopic {
  slug: string
  title: string
  summary?: string
  intent?: string
  category?: string
  order?: number
  icon?: string
  relatedPaths?: string[]
  relatedTopics?: string[]
  body?: string
}

// Lightweight summary from `/api/v1/help/topics`.
export interface HelpTopicSummary {
  slug: string
  title: string
  summary?: string
  category?: string
  order: number
  icon?: string
  relatedPaths?: string[]
}

export async function fetchHelpTopics(): Promise<HelpTopicSummary[]> {
  return api.get<HelpTopicSummary[]>('/help/topics')
}

export async function fetchHelpTopic(slug: string): Promise<HelpTopic> {
  return api.get<HelpTopic>(`/help/topics/${encodeURIComponent(slug)}`)
}

// ---- Path -> slug resolution (for FeatureHelpButton) ----
// We keep a fallback static map so the Help button always has something to open
// even before the topic list has loaded. The backend's `relatedPaths` frontmatter
// is the source of truth and overrides these defaults once topics are cached.

const FALLBACK_PATH_TOPIC_RULES: Array<[RegExp, string]> = [
  [/^\/servers\b/, 'servers'],
  [/^\/checks\b/, 'checks'],
  [/^\/incidents\b/, 'incidents'],
  [/^\/alerts\b/, 'alert-rules'],
  [/^\/alert-rules\b/, 'alert-rules'],
  [/^\/heartbeats\b/, 'heartbeats'],
  [/^\/maintenance\b/, 'maintenance-windows'],
  [/^\/mysql\/connections\b/, 'mysql-connections'],
  [/^\/mysql\/queries\b/, 'mysql-queries'],
  [/^\/mysql\/threads\b/, 'mysql-threads'],
  [/^\/mysql\/server\b/, 'mysql-server'],
  [/^\/mysql\b/, 'mysql'],
  [/^\/analytics\b/, 'analytics'],
  [/^\/status\b/, 'status-pages'],
  [/^\/rca\b/, 'rca-reports'],
  [/^\/ai\b/, 'ai-results'],
  [/^\/assistant\b/, 'assistant'],
  [/^\/recommendations\b/, 'monitor-tuning'],
  [/^\/automation\b/, 'remediation'],
  [/^\/logs\b/, 'log-events'],
  [/^\/notifications\b/, 'notifications'],
  [/^\/users\b/, 'users'],
  [/^\/settings\b/, 'settings'],
  [/^\/help\b/, 'getting-started'],
  [/^\/login\b/, 'login'],
  [/^\/$/, 'dashboard'],
]

// Dynamic rules built from `relatedPaths` in the loaded topic list.
let dynamicRules: Array<[RegExp, string]> = []

export function setHelpTopicsForRouting(topics: HelpTopicSummary[]) {
  const rules: Array<[RegExp, string]> = []
  for (const t of topics) {
    if (!t.relatedPaths) continue
    for (const p of t.relatedPaths) {
      try {
        const escaped = p
          .replace(/[-/\\^$+?.()|[\]{}]/g, '\\$&')
          .replace(/\\\*/g, '.*')
        rules.push([new RegExp('^' + escaped), t.slug])
      } catch {
        /* ignore malformed path patterns */
      }
    }
  }
  dynamicRules = rules
}

export function getHelpSlugForPath(pathname: string): string {
  const dyn = dynamicRules.find(([pattern]) => pattern.test(pathname))
  if (dyn) return dyn[1]
  return (
    FALLBACK_PATH_TOPIC_RULES.find(([pattern]) => pattern.test(pathname))?.[1] ??
    'getting-started'
  )
}
