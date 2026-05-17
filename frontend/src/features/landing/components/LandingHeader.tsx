import { Link } from 'react-router-dom'
import { Github, Heart, Lock } from 'lucide-react'
import { repoUrl } from '@/features/landing/landingData'

export function LandingHeader() {
  return (
    <header className="sticky top-0 z-40 border-b border-slate-200 bg-white/95 shadow-sm shadow-slate-950/5 backdrop-blur-xl">
      <div className="mx-auto flex h-[72px] max-w-[1440px] items-center justify-between px-4 sm:px-6 lg:px-8">
        <a href="#top" className="flex items-center gap-3" aria-label="HealthOps home">
          <span className="flex h-9 w-9 items-center justify-center rounded-[8px] bg-slate-950 text-white shadow-lg shadow-slate-950/20">
            <Heart className="h-[18px] w-[18px]" />
          </span>
          <span className="text-base font-bold tracking-normal text-slate-950">HealthOps</span>
        </a>

        <nav className="hidden items-center gap-5 text-sm font-medium text-slate-600 lg:flex">
          <a href="#capabilities" className="hover:text-slate-950">Capabilities</a>
          <a href="#screens" className="hover:text-slate-950">Screens</a>
          <a href="#auto-heal" className="hover:text-slate-950">Auto-Heal</a>
          <a href="#engineering" className="hover:text-slate-950">Engineering</a>
          <a href="#replacement" className="hover:text-slate-950">Compare</a>
          <Link to="/help/getting-started" className="hover:text-slate-950">Docs</Link>
        </nav>

        <div className="flex items-center gap-2">
          <a
            href={repoUrl}
            target="_blank"
            rel="noreferrer"
            className="hidden items-center gap-2 rounded-[8px] border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-slate-800 shadow-sm transition hover:border-slate-400 hover:bg-slate-50 sm:inline-flex"
          >
            <Github className="h-4 w-4" />
            GitHub
          </a>
          <Link
            to="/login"
            className="inline-flex items-center gap-2 rounded-[8px] bg-slate-950 px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-800"
          >
            Sign in
            <Lock className="h-4 w-4" />
          </Link>
        </div>
      </div>
    </header>
  )
}
