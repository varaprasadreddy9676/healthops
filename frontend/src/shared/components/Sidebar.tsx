import { NavLink, useLocation } from 'react-router-dom'
import { useState } from 'react'
import {
  LayoutDashboard, Activity, AlertTriangle, Database,
  BarChart3, Brain, Settings, ChevronsLeft, X, Heart, Server,
  Users, Bell, LogOut, FileText, GitBranch, MessageSquare, Eye, Zap, Globe2,
  ChevronDown,
} from 'lucide-react'
import { cn } from "@/shared/lib/utils"
import { useAuth } from "@/shared/hooks/useAuth"
import { useAIAvailability } from "@/features/ai/hooks/useAIAvailability"

const ICON_MAP = {
  LayoutDashboard, Activity, AlertTriangle, Database,
  BarChart3, Brain, Settings, Server, Users, Bell, FileText, GitBranch, MessageSquare, Eye, Zap, Globe2,
} as const

type IconName = keyof typeof ICON_MAP

interface NavItem {
  label: string
  path: string
  icon: IconName
  requiresAI?: boolean
}

interface NavSection {
  title: string
  items: NavItem[]
}

const NAV_SECTIONS: NavSection[] = [
  {
    title: '',
    items: [
      { label: 'Dashboard', path: '/', icon: 'LayoutDashboard' },
    ],
  },
  {
    title: 'Monitor',
    items: [
      { label: 'Servers', path: '/servers', icon: 'Server' },
      { label: 'Checks', path: '/checks', icon: 'Activity' },
      { label: 'MySQL', path: '/mysql', icon: 'Database' },
      { label: 'Log Events', path: '/logs', icon: 'FileText' },
      { label: 'Analytics', path: '/analytics', icon: 'BarChart3' },
    ],
  },
  {
    title: 'Respond',
    items: [
      { label: 'Incidents', path: '/incidents', icon: 'AlertTriangle' },
      { label: 'Root Cause', path: '/rca', icon: 'GitBranch', requiresAI: true },
      { label: 'Remediation', path: '/automation', icon: 'Zap', requiresAI: true },
      { label: 'Status Pages', path: '/status-pages', icon: 'Globe2' },
    ],
  },
  {
    title: 'Intelligence',
    items: [
      { label: 'AI Results', path: '/ai', icon: 'Brain', requiresAI: true },
      { label: 'Ask AI', path: '/assistant', icon: 'MessageSquare', requiresAI: true },
      { label: 'Tuning', path: '/recommendations', icon: 'Eye' },
    ],
  },
  {
    title: 'Configure',
    items: [
      { label: 'Notifications', path: '/notifications', icon: 'Bell' },
      { label: 'Users', path: '/users', icon: 'Users' },
      { label: 'Settings', path: '/settings', icon: 'Settings' },
    ],
  },
]

interface Props {
  collapsed: boolean
  mobileOpen: boolean
  onCollapse: () => void
  onMobileClose: () => void
}

export function Sidebar({ collapsed, mobileOpen, onCollapse, onMobileClose }: Props) {
  const { user, logout } = useAuth()
  const { isAIAvailable } = useAIAvailability()
  const location = useLocation()
  const [collapsedSections, setCollapsedSections] = useState<Record<string, boolean>>({})

  const toggleSection = (title: string) => {
    setCollapsedSections(prev => ({ ...prev, [title]: !prev[title] }))
  }

  const sections = NAV_SECTIONS.map(section => ({
    ...section,
    items: section.items.filter(item => !item.requiresAI || isAIAvailable),
  })).filter(section => section.items.length > 0)

  return (
    <aside
      className={cn(
        'fixed inset-y-0 left-0 z-50 flex flex-col border-r border-slate-200 bg-white transition-all duration-200 ease-out dark:border-slate-800 dark:bg-slate-900',
        'lg:relative lg:z-auto',
        collapsed ? 'lg:w-[68px]' : 'lg:w-60',
        mobileOpen ? 'w-60 translate-x-0' : '-translate-x-full lg:translate-x-0',
      )}
    >
      {/* Brand */}
      <div className="flex h-14 items-center gap-3 border-b border-slate-200 px-4 dark:border-slate-800">
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-blue-600 text-white">
          <Heart className="h-4 w-4" />
        </div>
        {!collapsed && (
          <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">
            HealthOps
          </span>
        )}
        {/* Mobile close */}
        <button
          onClick={onMobileClose}
          className="ml-auto rounded-md p-1 text-slate-400 hover:text-slate-600 lg:hidden"
          aria-label="Close navigation"
        >
          <X className="h-5 w-5" />
        </button>
        {/* Desktop collapse */}
        <button
          onClick={onCollapse}
          className={cn(
            'ml-auto hidden rounded-md p-1 text-slate-400 transition-colors hover:text-slate-600 lg:block',
            collapsed && 'ml-0',
          )}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          <ChevronsLeft className={cn('h-4 w-4 transition-transform', collapsed && 'rotate-180')} />
        </button>
      </div>

      {/* Nav */}
      <nav className="flex-1 overflow-y-auto px-2 py-3">
        {sections.map((section) => {
          const isOpen = !collapsedSections[section.title]
          const sectionActive = section.items.some(item =>
            item.path === '/' ? location.pathname === '/' : location.pathname.startsWith(item.path)
          )

          return (
            <div key={section.title || '_root'} className={section.title ? 'mt-3' : ''}>
              {/* Section header */}
              {section.title && !collapsed && (
                <button
                  onClick={() => toggleSection(section.title)}
                  className="mb-0.5 flex w-full items-center justify-between px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300"
                >
                  {section.title}
                  <ChevronDown className={cn('h-3 w-3 transition-transform', !isOpen && '-rotate-90')} />
                </button>
              )}
              {section.title && collapsed && (
                <div className="mx-auto my-2 h-px w-6 bg-slate-200 dark:bg-slate-700" />
              )}

              {/* Section items */}
              {(isOpen || collapsed) && (
                <div className="space-y-0.5">
                  {section.items.map((item) => {
                    const Icon = ICON_MAP[item.icon]
                    return (
                      <NavLink
                        key={item.path}
                        to={item.path}
                        end={item.path === '/'}
                        onClick={onMobileClose}
                        className={({ isActive }) =>
                          cn(
                            'group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                            isActive
                              ? 'bg-blue-50 text-blue-700 dark:bg-blue-950/50 dark:text-blue-300'
                              : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200',
                            collapsed && 'justify-center px-2',
                          )
                        }
                      >
                        <Icon className="h-[18px] w-[18px] shrink-0" />
                        {!collapsed && <span>{item.label}</span>}
                      </NavLink>
                    )
                  })}
                </div>
              )}

              {/* Collapsed indicator for hidden section with active item */}
              {!isOpen && !collapsed && sectionActive && (
                <div className="ml-3 h-0.5 w-4 rounded bg-blue-500" />
              )}
            </div>
          )
        })}
      </nav>

      {/* Footer */}
      {!collapsed && (
        <div className="border-t border-slate-200 px-3 py-3 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-slate-700 dark:text-slate-300">{user?.displayName || user?.username}</p>
              <p className="text-xs text-slate-400 capitalize">{user?.role}</p>
            </div>
            <button
              onClick={logout}
              className="shrink-0 rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800"
              title="Logout"
            >
              <LogOut className="h-4 w-4" />
            </button>
          </div>
        </div>
      )}
      {collapsed && (
        <div className="border-t border-slate-200 px-2 py-3 dark:border-slate-800">
          <button
            onClick={logout}
            className="mx-auto flex h-8 w-8 items-center justify-center rounded-md text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800"
            title="Logout"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      )}
    </aside>
  )
}
