import { useMemo } from 'react'
import { cn } from '@/shared/lib/utils'
import { TrendingUp, TrendingDown, Minus, Zap } from 'lucide-react'
import type { SignalSeries } from '@/features/incidents/api/rca'

const TREND_CONFIG: Record<string, { icon: typeof TrendingUp; color: string; label: string }> = {
    rising: { icon: TrendingUp, color: 'text-red-500', label: 'Rising' },
    falling: { icon: TrendingDown, color: 'text-emerald-500', label: 'Falling' },
    stable: { icon: Minus, color: 'text-slate-400', label: 'Stable' },
    spike: { icon: Zap, color: 'text-amber-500', label: 'Spike' },
}

function Sparkline({ points, trend }: { points: { timestamp: string; value: number }[]; trend: string }) {
    const { path, area, width, height } = useMemo(() => {
        const w = 140
        const h = 32
        if (!points || points.length < 2) return { path: '', area: '', width: w, height: h }

        const values = points.map((p) => p.value)
        const min = Math.min(...values)
        const max = Math.max(...values)
        const range = max - min || 1

        const coords = values.map((v, i) => ({
            x: (i / (values.length - 1)) * w,
            y: h - ((v - min) / range) * (h - 4) - 2,
        }))

        const linePath = coords.map((c, i) => `${i === 0 ? 'M' : 'L'}${c.x.toFixed(1)},${c.y.toFixed(1)}`).join(' ')
        const areaPath = `${linePath} L${w},${h} L0,${h} Z`

        return { path: linePath, area: areaPath, width: w, height: h }
    }, [points])

    const strokeColor = trend === 'spike' ? '#f59e0b' : trend === 'rising' ? '#ef4444' : trend === 'falling' ? '#10b981' : '#94a3b8'
    const fillColor = trend === 'spike' ? '#fef3c7' : trend === 'rising' ? '#fef2f2' : trend === 'falling' ? '#ecfdf5' : '#f8fafc'

    if (!path) {
        return <div className="h-8 w-[140px] rounded bg-slate-100 dark:bg-slate-800" />
    }

    return (
        <svg width={width} height={height} className="overflow-visible">
            <path d={area} fill={fillColor} opacity={0.5} />
            <path d={path} fill="none" stroke={strokeColor} strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" />
        </svg>
    )
}

export function SignalCard({ signal }: { signal: SignalSeries }) {
    const config = TREND_CONFIG[signal.trend] || TREND_CONFIG.stable
    const TrendIcon = config.icon

    return (
        <div className="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-800/60">
            <div className="flex items-center justify-between gap-2 mb-2">
                <div className="min-w-0">
                    <p className="truncate text-xs font-medium text-slate-700 dark:text-slate-300">{signal.name}</p>
                    <p className="truncate text-[10px] text-slate-400">{signal.source}{signal.server ? ` / ${signal.server}` : ''}</p>
                </div>
                <div className={cn('flex items-center gap-1 shrink-0', config.color)}>
                    <TrendIcon className="h-3 w-3" />
                    <span className="text-[10px] font-semibold">{config.label}</span>
                </div>
            </div>
            <Sparkline points={signal.points} trend={signal.trend} />
            <div className="mt-1.5 flex items-center gap-3 text-[10px] text-slate-400">
                <span>min: {signal.min.toFixed(1)}</span>
                <span>avg: {signal.avg.toFixed(1)}</span>
                <span>max: {signal.max.toFixed(1)}</span>
            </div>
        </div>
    )
}

export function SignalGrid({ signals }: { signals: SignalSeries[] }) {
    if (!signals || signals.length === 0) return null

    // Sort: spikes first, then rising, then falling, then stable
    const sortOrder: Record<string, number> = { spike: 0, rising: 1, falling: 2, stable: 3 }
    const sorted = [...signals].sort((a, b) => (sortOrder[a.trend] ?? 4) - (sortOrder[b.trend] ?? 4))

    return (
        <div className="space-y-2">
            <h3 className="text-xs font-semibold uppercase text-slate-500">
                Correlated Signals ({signals.length})
            </h3>
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {sorted.map((signal, i) => (
                    <SignalCard key={`${signal.name}-${signal.source}-${i}`} signal={signal} />
                ))}
            </div>
        </div>
    )
}
