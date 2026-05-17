import { Link, useParams } from 'react-router-dom'
import {
  ArrowLeft,
  BookOpen,
  Check,
  Copy,
  ExternalLink,
  Hash,
  Info,
  List,
  Loader2,
  Search,
  Sparkles,
} from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  fetchHelpTopic,
  fetchHelpTopics,
  setHelpTopicsForRouting,
  type HelpTopicSummary,
} from '@/features/help/helpContent'
import { cn } from '@/shared/lib/utils'

const DEFAULT_SLUG = 'getting-started'
const CATEGORY_ORDER = [
  'Start Here',
  'Operate',
  'Detect',
  'Understand',
  'AI',
  'Admin',
  'Scenarios',
] as const

export default function HelpPage() {
  const { slug: slugParam } = useParams<{ slug?: string }>()
  const slug = slugParam || DEFAULT_SLUG
  const [search, setSearch] = useState('')
  const searchRef = useRef<HTMLInputElement>(null)

  const topicsQuery = useQuery({
    queryKey: ['help', 'topics'],
    queryFn: fetchHelpTopics,
    staleTime: 1000 * 60 * 30,
  })

  const topicQuery = useQuery({
    queryKey: ['help', 'topic', slug],
    queryFn: () => fetchHelpTopic(slug),
    enabled: !!slug,
    staleTime: 1000 * 60 * 30,
  })

  useEffect(() => {
    if (topicsQuery.data) setHelpTopicsForRouting(topicsQuery.data)
  }, [topicsQuery.data])

  // Cmd/Ctrl+K focuses the sidebar search.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        searchRef.current?.focus()
        searchRef.current?.select()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // Scroll article to top on slug change.
  const articleRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    articleRef.current?.scrollTo?.({ top: 0 })
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [slug])

  const allTopics = topicsQuery.data ?? []
  const titleBySlug = useMemo(() => {
    const m = new Map<string, string>()
    for (const t of allTopics) m.set(t.slug, t.title)
    return m
  }, [allTopics])

  const filteredTopics = useMemo(() => {
    if (!search.trim()) return allTopics
    const q = search.trim().toLowerCase()
    return allTopics.filter(
      (t) =>
        t.title.toLowerCase().includes(q) ||
        (t.summary ?? '').toLowerCase().includes(q) ||
        t.slug.toLowerCase().includes(q),
    )
  }, [allTopics, search])

  const groupedTopics = useMemo(() => groupByCategory(filteredTopics), [filteredTopics])

  const topic = topicQuery.data
  const isLoadingTopic = topicQuery.isPending

  // Extract h2 sections from the body for the right-rail TOC.
  const sections = useMemo(() => extractSections(topic?.body ?? ''), [topic?.body])

  return (
    <div className="mx-auto max-w-[1400px] animate-fade-in px-1">
      {/* Top bar */}
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Link
            to="/"
            className="inline-flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-slate-800 dark:hover:text-slate-100"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to app
          </Link>
          <div className="hidden items-center gap-2 text-xs text-slate-400 sm:flex">
            <span>/</span>
            <span className="font-medium text-slate-600 dark:text-slate-300">Help</span>
            {topic?.category && (
              <>
                <span>/</span>
                <span>{topic.category}</span>
              </>
            )}
            {topic && (
              <>
                <span>/</span>
                <span className="font-medium text-slate-900 dark:text-slate-100">{topic.title}</span>
              </>
            )}
          </div>
        </div>
        <a
          href={`/help/${slug}`}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 shadow-sm hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          Open in new tab
          <ExternalLink className="h-3.5 w-3.5" />
        </a>
      </div>

      <div className="grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)_220px]">
        {/* ---- Left sidebar ---- */}
        <aside className="lg:sticky lg:top-4 lg:max-h-[calc(100vh-2rem)] lg:self-start lg:overflow-y-auto">
          <div className="rounded-xl border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-800 dark:bg-slate-900">
            <div className="mb-3 flex items-center gap-2 px-1 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
              <BookOpen className="h-3.5 w-3.5" />
              Documentation
            </div>

            {/* Search */}
            <div className="relative mb-3">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <input
                ref={searchRef}
                type="search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search topics..."
                className="w-full rounded-lg border border-slate-200 bg-slate-50/60 py-1.5 pl-8 pr-12 text-sm text-slate-900 placeholder-slate-400 outline-none transition focus:border-blue-400 focus:bg-white focus:ring-2 focus:ring-blue-100 dark:border-slate-700 dark:bg-slate-950/40 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:bg-slate-950 dark:focus:ring-blue-900/40"
              />
              <kbd className="pointer-events-none absolute right-2 top-1/2 hidden -translate-y-1/2 rounded border border-slate-200 bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500 sm:inline-block dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400">
                ⌘K
              </kbd>
            </div>

            {topicsQuery.isPending && (
              <div className="px-2 py-3 text-xs text-slate-500">
                <Loader2 className="mr-1.5 inline h-3.5 w-3.5 animate-spin" />
                Loading topics...
              </div>
            )}
            {topicsQuery.error && (
              <div className="rounded-lg border border-rose-200 bg-rose-50 px-2 py-2 text-xs text-rose-600 dark:border-rose-900/40 dark:bg-rose-950/30 dark:text-rose-400">
                Failed to load topics
              </div>
            )}

            <nav className="space-y-4">
              {groupedTopics.map((group) => (
                <div key={group.category}>
                  <div className="mb-1.5 flex items-center gap-1.5 px-2 text-[10px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">
                    {group.category}
                    <span className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[9px] text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                      {group.topics.length}
                    </span>
                  </div>
                  <div className="space-y-0.5">
                    {group.topics.map((item) => (
                      <Link
                        key={item.slug}
                        to={`/help/${item.slug}`}
                        className={cn(
                          'group flex items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm font-medium transition-colors',
                          item.slug === slug
                            ? 'bg-blue-50 text-blue-700 shadow-sm ring-1 ring-blue-100 dark:bg-blue-950/40 dark:text-blue-300 dark:ring-blue-900/40'
                            : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100',
                        )}
                      >
                        <span
                          className={cn(
                            'h-1.5 w-1.5 shrink-0 rounded-full transition',
                            item.slug === slug ? 'bg-blue-500' : 'bg-slate-300 dark:bg-slate-600',
                          )}
                        />
                        <span className="truncate">{item.title}</span>
                      </Link>
                    ))}
                  </div>
                </div>
              ))}
              {!topicsQuery.isPending && groupedTopics.length === 0 && (
                <div className="px-2 py-6 text-center text-xs text-slate-500">
                  No topics match "{search}"
                </div>
              )}
            </nav>
          </div>
        </aside>

        {/* ---- Main article ---- */}
        <article
          ref={articleRef}
          className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
        >
          {isLoadingTopic && (
            <div className="flex items-center gap-2 px-8 py-16 text-sm text-slate-500">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading topic...
            </div>
          )}

          {topicQuery.error && (
            <div className="px-8 py-16">
              <h1 className="text-xl font-semibold text-slate-900 dark:text-slate-100">
                Topic not found
              </h1>
              <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
                The help topic "{slug}" could not be loaded. Pick a topic from the sidebar.
              </p>
              <Link
                to="/help"
                className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
              >
                Go to Getting Started
              </Link>
            </div>
          )}

          {topic && (
            <>
              <header className="border-b border-slate-100 bg-gradient-to-b from-slate-50/50 to-transparent px-8 py-7 dark:border-slate-800 dark:from-slate-950/50">
                <div className="mb-3 flex flex-wrap items-center gap-2">
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wide text-blue-700 ring-1 ring-blue-100 dark:bg-blue-950/40 dark:text-blue-300 dark:ring-blue-900/40">
                    <Sparkles className="h-3 w-3" />
                    {topic.category || 'Guide'}
                  </span>
                  <span className="font-mono text-[10px] text-slate-400">{topic.slug}</span>
                </div>
                <h1 className="text-3xl font-bold tracking-tight text-slate-950 dark:text-slate-50">
                  {topic.title}
                </h1>
                {topic.summary && (
                  <p className="mt-3 max-w-3xl text-base leading-7 text-slate-600 dark:text-slate-300">
                    {topic.summary}
                  </p>
                )}
                {topic.intent && (
                  <div className="mt-4 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50/60 px-3 py-2.5 text-sm text-amber-900 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-200">
                    <Info className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
                    <div>
                      <span className="font-semibold">When to use this:</span> {topic.intent}
                    </div>
                  </div>
                )}
              </header>

              <div className="px-8 py-7">
                <div className="prose prose-slate max-w-none dark:prose-invert prose-headings:scroll-mt-24 prose-headings:font-semibold prose-h1:hidden prose-h2:mt-10 prose-h2:border-b prose-h2:border-slate-100 prose-h2:pb-2 prose-h2:text-xl dark:prose-h2:border-slate-800 prose-h3:mt-8 prose-h3:text-lg prose-p:leading-7 prose-a:font-medium prose-a:text-blue-600 prose-a:no-underline hover:prose-a:underline dark:prose-a:text-blue-400 prose-strong:text-slate-900 dark:prose-strong:text-slate-100 prose-code:rounded prose-code:border prose-code:border-slate-200 prose-code:bg-slate-50 prose-code:px-1.5 prose-code:py-0.5 prose-code:text-[0.85em] prose-code:font-medium prose-code:font-mono prose-code:before:content-none prose-code:after:content-none dark:prose-code:border-slate-700 dark:prose-code:bg-slate-800 dark:prose-code:text-slate-200 prose-pre:overflow-hidden prose-pre:rounded-lg prose-pre:border prose-pre:border-slate-800 prose-pre:bg-slate-950 prose-pre:p-0 prose-pre:text-slate-100 prose-blockquote:border-l-blue-400 prose-blockquote:bg-blue-50/40 prose-blockquote:py-1 prose-blockquote:not-italic prose-blockquote:text-slate-700 dark:prose-blockquote:bg-blue-950/20 dark:prose-blockquote:text-slate-300 prose-table:overflow-hidden prose-table:rounded-lg prose-table:border prose-table:border-slate-200 prose-thead:bg-slate-50 prose-th:px-3 prose-th:py-2 prose-th:text-left prose-th:text-xs prose-th:font-semibold prose-th:uppercase prose-th:tracking-wide prose-th:text-slate-600 prose-td:border-t prose-td:border-slate-100 prose-td:px-3 prose-td:py-2 prose-td:text-sm dark:prose-table:border-slate-800 dark:prose-thead:bg-slate-900 dark:prose-th:text-slate-400 dark:prose-td:border-slate-800 prose-li:my-1 prose-ul:my-3 prose-ol:my-3">
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    components={{
                      h2({ children }) {
                        const text = childrenToText(children)
                        const id = slugifyHeading(text)
                        return (
                          <h2 id={id} className="group/h relative">
                            <a
                              href={`#${id}`}
                              className="absolute -left-6 top-1/2 -translate-y-1/2 text-slate-300 opacity-0 transition-opacity group-hover/h:opacity-100 hover:text-blue-500"
                              aria-label="Link to section"
                            >
                              <Hash className="h-4 w-4" />
                            </a>
                            {children}
                          </h2>
                        )
                      },
                      h3({ children }) {
                        const text = childrenToText(children)
                        const id = slugifyHeading(text)
                        return (
                          <h3 id={id} className="group/h relative">
                            <a
                              href={`#${id}`}
                              className="absolute -left-6 top-1/2 -translate-y-1/2 text-slate-300 opacity-0 transition-opacity group-hover/h:opacity-100 hover:text-blue-500"
                              aria-label="Link to section"
                            >
                              <Hash className="h-3.5 w-3.5" />
                            </a>
                            {children}
                          </h3>
                        )
                      },
                      pre({ children }) {
                        return <CodeBlock>{children}</CodeBlock>
                      },
                      a({ href, children, ...props }) {
                        const isExternal = href?.startsWith('http')
                        return (
                          <a
                            href={href}
                            target={isExternal ? '_blank' : undefined}
                            rel={isExternal ? 'noreferrer' : undefined}
                            {...props}
                          >
                            {children}
                            {isExternal && <ExternalLink className="ml-0.5 inline h-3 w-3 align-text-top" />}
                          </a>
                        )
                      },
                    }}
                  >
                    {topic.body || ''}
                  </ReactMarkdown>
                </div>

                {topic.relatedTopics && topic.relatedTopics.length > 0 && (
                  <div className="mt-10 border-t border-slate-100 pt-6 dark:border-slate-800">
                    <div className="mb-3 text-xs font-bold uppercase tracking-wider text-slate-500">
                      Related Topics
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {topic.relatedTopics.map((s) => (
                        <Link
                          key={s}
                          to={`/help/${s}`}
                          className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 shadow-sm transition hover:-translate-y-0.5 hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700 hover:shadow dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-blue-700 dark:hover:bg-blue-950/40 dark:hover:text-blue-300"
                        >
                          <BookOpen className="h-3.5 w-3.5" />
                          {titleBySlug.get(s) || s}
                        </Link>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </>
          )}
        </article>

        {/* ---- Right rail TOC ---- */}
        <aside className="hidden lg:sticky lg:top-4 lg:block lg:max-h-[calc(100vh-2rem)] lg:self-start lg:overflow-y-auto">
          {sections.length > 1 && (
            <div className="rounded-xl border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-800 dark:bg-slate-900">
              <div className="mb-2 flex items-center gap-1.5 px-1 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                <List className="h-3.5 w-3.5" />
                On this page
              </div>
              <nav className="space-y-0.5">
                {sections.map((s) => (
                  <a
                    key={s.id}
                    href={`#${s.id}`}
                    className="block truncate rounded px-2 py-1 text-xs text-slate-500 transition hover:bg-slate-50 hover:text-blue-600 dark:hover:bg-slate-800 dark:hover:text-blue-400"
                  >
                    {s.title}
                  </a>
                ))}
              </nav>
            </div>
          )}
        </aside>
      </div>
    </div>
  )
}

// --- Helpers ---

interface TopicGroup {
  category: string
  topics: HelpTopicSummary[]
}

function groupByCategory(topics: HelpTopicSummary[]): TopicGroup[] {
  const map = new Map<string, HelpTopicSummary[]>()
  for (const t of topics) {
    const cat = t.category || 'Other'
    if (!map.has(cat)) map.set(cat, [])
    map.get(cat)!.push(t)
  }
  // Sort within each category by order then title.
  for (const arr of map.values()) {
    arr.sort((a, b) => (a.order ?? 999) - (b.order ?? 999) || a.title.localeCompare(b.title))
  }
  const groups: TopicGroup[] = []
  for (const cat of CATEGORY_ORDER) {
    if (map.has(cat)) groups.push({ category: cat, topics: map.get(cat)! })
  }
  for (const [cat, items] of map.entries()) {
    if (!CATEGORY_ORDER.includes(cat as (typeof CATEGORY_ORDER)[number])) {
      groups.push({ category: cat, topics: items })
    }
  }
  return groups
}

function slugifyHeading(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-')
}

function childrenToText(node: React.ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(childrenToText).join('')
  if (node && typeof node === 'object' && 'props' in node) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return childrenToText((node as any).props.children)
  }
  return ''
}

function extractSections(body: string): { id: string; title: string }[] {
  const out: { id: string; title: string }[] = []
  const lines = body.split('\n')
  let inFence = false
  for (const line of lines) {
    if (line.startsWith('```')) {
      inFence = !inFence
      continue
    }
    if (inFence) continue
    const m = /^## (.+?)\s*$/.exec(line)
    if (m) {
      const title = m[1].trim()
      out.push({ id: slugifyHeading(title), title })
    }
  }
  return out
}

// Code block with a Copy button overlay.
function CodeBlock({ children }: { children: React.ReactNode }) {
  const [copied, setCopied] = useState(false)
  const text = useMemo(() => extractCodeText(children), [children])

  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      /* ignore */
    }
  }

  return (
    <div className="not-prose group/code relative my-4 overflow-hidden rounded-lg border border-slate-800 bg-slate-950">
      <button
        type="button"
        onClick={onCopy}
        className="absolute right-2 top-2 z-10 inline-flex items-center gap-1 rounded-md border border-slate-700 bg-slate-900/80 px-2 py-1 text-[11px] font-medium text-slate-300 opacity-0 backdrop-blur transition hover:bg-slate-800 hover:text-white group-hover/code:opacity-100"
        aria-label="Copy code"
      >
        {copied ? (
          <>
            <Check className="h-3 w-3 text-emerald-400" />
            Copied
          </>
        ) : (
          <>
            <Copy className="h-3 w-3" />
            Copy
          </>
        )}
      </button>
      <pre className="overflow-x-auto p-4 text-[13px] leading-relaxed text-slate-100">{children}</pre>
    </div>
  )
}

function extractCodeText(node: React.ReactNode): string {
  if (typeof node === 'string') return node
  if (typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(extractCodeText).join('')
  if (node && typeof node === 'object' && 'props' in node) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return extractCodeText((node as any).props.children)
  }
  return ''
}
