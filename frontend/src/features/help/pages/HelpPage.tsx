import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, BookOpen, ExternalLink, Info } from 'lucide-react'
import { HELP_TOPICS, HELP_TOPIC_BY_SLUG } from '@/features/help/helpContent'
import { cn } from '@/shared/lib/utils'

export default function HelpPage() {
  const { slug } = useParams<{ slug?: string }>()
  const topic = HELP_TOPIC_BY_SLUG[slug || 'dashboard'] ?? HELP_TOPIC_BY_SLUG.dashboard

  return (
    <div className="mx-auto max-w-6xl animate-fade-in">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <Link to="/" className="inline-flex items-center gap-1.5 text-sm font-medium text-slate-500 hover:text-slate-800 dark:hover:text-slate-200">
          <ArrowLeft className="h-4 w-4" />
          Back to app
        </Link>
        <a
          href={`/help/${topic.slug}`}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          Open clean tab
          <ExternalLink className="h-3.5 w-3.5" />
        </a>
      </div>

      <div className="grid gap-6 lg:grid-cols-[260px_minmax(0,1fr)]">
        <aside className="lg:sticky lg:top-4 lg:self-start">
          <div className="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900">
            <div className="mb-3 flex items-center gap-2 px-2 text-xs font-semibold uppercase tracking-wide text-slate-500">
              <BookOpen className="h-4 w-4" />
              Feature Guides
            </div>
            <nav className="space-y-1">
              {HELP_TOPICS.map((item) => (
                <Link
                  key={item.slug}
                  to={`/help/${item.slug}`}
                  className={cn(
                    'block rounded-lg px-2.5 py-2 text-sm font-medium transition-colors',
                    item.slug === topic.slug
                      ? 'bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300'
                      : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100',
                  )}
                >
                  {item.title}
                </Link>
              ))}
            </nav>
          </div>
        </aside>

        <article className="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
          <header className="border-b border-slate-100 px-6 py-6 dark:border-slate-800">
            <div className="mb-3 inline-flex items-center gap-2 rounded-full bg-blue-50 px-3 py-1 text-xs font-semibold text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">
              <Info className="h-3.5 w-3.5" />
              HealthOps guide
            </div>
            <h1 className="text-2xl font-bold text-slate-950 dark:text-slate-50">{topic.title}</h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-600 dark:text-slate-300">{topic.summary}</p>
            <p className="mt-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-700 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-300">
              <span className="font-semibold text-slate-900 dark:text-slate-100">When to use it:</span> {topic.intent}
            </p>
          </header>

          <div className="divide-y divide-slate-100 dark:divide-slate-800">
            {topic.sections.map((section) => (
              <section key={section.title} className="px-6 py-5">
                <h2 className="text-base font-semibold text-slate-900 dark:text-slate-100">{section.title}</h2>
                {section.paragraphs?.map((paragraph) => (
                  <p key={paragraph} className="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
                    {paragraph}
                  </p>
                ))}
                {section.bullets && (
                  <ul className="mt-3 space-y-2">
                    {section.bullets.map((bullet) => (
                      <li key={bullet} className="flex gap-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
                        <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-blue-500" />
                        <span>{bullet}</span>
                      </li>
                    ))}
                  </ul>
                )}
                {section.code && (
                  <div className="mt-4 overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
                    <div className="border-b border-slate-200 bg-slate-50 px-3 py-2 text-xs font-semibold text-slate-500 dark:border-slate-800 dark:bg-slate-950">
                      {section.code.label}
                    </div>
                    <pre className="overflow-x-auto bg-slate-950 p-4 text-xs leading-6 text-slate-100">
                      <code>{section.code.code}</code>
                    </pre>
                  </div>
                )}
              </section>
            ))}
          </div>
        </article>
      </div>
    </div>
  )
}
