import { Link } from 'react-router-dom'
import { Heart } from 'lucide-react'
import { footerLinks, ossBadges } from '@/features/landing/landingData'

export function Footer() {
  return (
    <footer className="bg-slate-950 py-10 text-slate-300">
      <div className="mx-auto max-w-[1440px] px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col gap-6 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="flex items-center gap-3">
              <span className="flex h-9 w-9 items-center justify-center rounded-[8px] bg-white text-slate-950">
                <Heart className="h-[18px] w-[18px]" />
              </span>
              <span className="text-base font-bold text-white">HealthOps</span>
            </div>
            <p className="mt-3 max-w-xl text-sm leading-6 text-slate-400">
              Self-hosted monitoring for small ops teams.
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
              {ossBadges.map((badge) => (
                <span key={badge} className="rounded-[6px] border border-white/10 px-2 py-1 text-xs font-medium text-slate-300">
                  {badge}
                </span>
              ))}
            </div>
          </div>

          <nav className="flex flex-wrap gap-3 text-sm">
            {footerLinks.map((link) => {
              const Icon = link.icon
              const content = (
                <>
                  {Icon && <Icon className="h-4 w-4" />}
                  {link.label}
                </>
              )
              return link.external ? (
                <a
                  key={link.label}
                  href={link.href}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-2 rounded-[8px] border border-white/10 px-3 py-2 font-medium text-slate-300 transition hover:border-white/20 hover:text-white"
                >
                  {content}
                </a>
              ) : (
                <Link
                  key={link.label}
                  to={link.href}
                  className="inline-flex items-center gap-2 rounded-[8px] border border-white/10 px-3 py-2 font-medium text-slate-300 transition hover:border-white/20 hover:text-white"
                >
                  {content}
                </Link>
              )
            })}
          </nav>
        </div>
      </div>
    </footer>
  )
}
