import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from "@/shared/components/Layout"
import { LoadingState } from "@/shared/components/LoadingState"
import { useAuth } from "@/shared/hooks/useAuth"

const Login = lazy(() => import('@/features/auth/pages/Login'))
const Dashboard = lazy(() => import('@/features/dashboard/pages/Dashboard'))
const Servers = lazy(() => import('@/features/servers/pages/Servers'))
const ServerDetail = lazy(() => import('@/features/servers/pages/ServerDetail'))
const Checks = lazy(() => import('@/features/checks/pages/Checks'))
const CheckDetail = lazy(() => import('@/features/checks/pages/CheckDetail'))
const Incidents = lazy(() => import('@/features/incidents/pages/Incidents'))
const IncidentDetail = lazy(() => import('@/features/incidents/pages/IncidentDetail'))
const MySQL = lazy(() => import('@/features/mysql/pages/MySQL'))
const MySQLConnections = lazy(() => import('@/features/mysql/pages/MySQLConnections'))
const MySQLQueries = lazy(() => import('@/features/mysql/pages/MySQLQueries'))
const MySQLThreads = lazy(() => import('@/features/mysql/pages/MySQLThreads'))
const MySQLServer = lazy(() => import('@/features/mysql/pages/MySQLServer'))
const Analytics = lazy(() => import('@/features/analytics/pages/Analytics'))
const AIAnalysis = lazy(() => import('@/features/ai/pages/AIAnalysis'))
const Settings = lazy(() => import('@/features/settings/pages/Settings'))
const UserManagement = lazy(() => import('@/features/users/pages/UserManagement'))
const NotificationChannels = lazy(() => import('@/features/notifications/pages/NotificationChannels'))
const NotFound = lazy(() => import('@/shared/components/NotFound'))

export default function App() {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingState message="Loading…" />
  }

  return (
    <Suspense fallback={<LoadingState message="Loading…" />}>
      <Routes>
        {!isAuthenticated ? (
          <>
            <Route path="/login" element={<Login />} />
            <Route path="*" element={<Navigate to="/login" replace />} />
          </>
        ) : (
          <Route element={<Layout />}>
            <Route index element={<Dashboard />} />
            <Route path="servers" element={<Servers />} />
            <Route path="servers/:id" element={<ServerDetail />} />
            <Route path="checks" element={<Checks />} />
            <Route path="checks/:id" element={<CheckDetail />} />
            <Route path="incidents" element={<Incidents />} />
            <Route path="incidents/:id" element={<IncidentDetail />} />
            <Route path="mysql" element={<MySQL />} />
            <Route path="mysql/connections" element={<MySQLConnections />} />
            <Route path="mysql/queries" element={<MySQLQueries />} />
            <Route path="mysql/threads" element={<MySQLThreads />} />
            <Route path="mysql/server" element={<MySQLServer />} />
            <Route path="analytics" element={<Analytics />} />
            <Route path="ai" element={<AIAnalysis />} />
            <Route path="settings" element={<Settings />} />
            <Route path="users" element={<UserManagement />} />
            <Route path="notifications" element={<NotificationChannels />} />
            <Route path="login" element={<Navigate to="/" replace />} />
            <Route path="*" element={<NotFound />} />
          </Route>
        )}
      </Routes>
    </Suspense>
  )
}
