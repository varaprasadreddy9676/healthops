import { Info } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { getHelpSlugForPath, HELP_TOPIC_BY_SLUG } from '@/features/help/helpContent'

export function FeatureHelpButton() {
  const location = useLocation()
  const slug = getHelpSlugForPath(location.pathname)
  const topic = HELP_TOPIC_BY_SLUG[slug]

  const openHelp = () => {
    const url = `${window.location.origin}/help/${slug}`
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  return (
    <button
      onClick={openHelp}
      className="inline-flex items-center gap-1.5 rounded-md border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-600 transition-colors hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-blue-800 dark:hover:bg-blue-950/40 dark:hover:text-blue-300"
      aria-label={`Open help for ${topic?.title ?? 'this feature'}`}
      title={`Open ${topic?.title ?? 'feature'} guide in a new tab`}
    >
      <Info className="h-4 w-4" />
      <span className="hidden sm:inline">Help</span>
    </button>
  )
}
