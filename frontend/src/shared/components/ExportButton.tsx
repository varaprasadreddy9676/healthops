import { Download, FileJson, FileSpreadsheet } from '@/shared/icons/lucide'
import { useState } from 'react'
import { api } from "@/shared/api/client"
import { useToast } from "@/shared/components/Toast"
import { cn } from "@/shared/lib/utils"

interface Props {
  onExportCSV?: () => void
  onExportJSON?: () => void
  downloadUrl?: string
  filename?: string
  className?: string
}

export function ExportButton({ onExportCSV, onExportJSON, downloadUrl, filename = 'healthops-export', className }: Props) {
  const [downloading, setDownloading] = useState(false)
  const toast = useToast()

  if (downloadUrl) {
    return (
      <button
        type="button"
        disabled={downloading}
        onClick={async () => {
          setDownloading(true)
          try {
            await api.download(downloadUrl, filename)
          } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Export failed')
          } finally {
            setDownloading(false)
          }
        }}
        className={cn(
          'inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition-colors hover:bg-slate-50 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700',
          className,
        )}
      >
        <Download className="h-3.5 w-3.5" />
        {downloading ? 'Exporting…' : 'Export'}
      </button>
    )
  }

  return (
    <div className={cn('flex items-center gap-1', className)}>
      {onExportCSV && (
        <button
          onClick={onExportCSV}
          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-600 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
        >
          <FileSpreadsheet className="h-3.5 w-3.5" />
          CSV
        </button>
      )}
      {onExportJSON && (
        <button
          onClick={onExportJSON}
          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-600 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
        >
          <FileJson className="h-3.5 w-3.5" />
          JSON
        </button>
      )}
    </div>
  )
}
