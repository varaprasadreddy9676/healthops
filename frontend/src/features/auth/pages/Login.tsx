import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircle, ArrowRight, Heart, KeyRound, Sparkles } from 'lucide-react'
import { useAuth } from '@/shared/hooks/useAuth'

interface ConfigResponse {
  data: {
    isDemoMode: boolean
  }
}

export default function Login() {
  const { login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [isDemoMode, setIsDemoMode] = useState(false)

  // Fetch config to check if demo mode
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const response = await fetch('/api/v1/config')
        const result: ConfigResponse = await response.json()
        setIsDemoMode(result.data.isDemoMode)
      } catch {
        setIsDemoMode(false)
      }
    }
    fetchConfig()
  }, [])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      await login(username, password)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  const fillDefaultCredentials = () => {
    setUsername('admin')
    setPassword('healthops-demo-admin')
    setError('')
  }

  return (
    <div className="landing-grid flex min-h-screen items-center justify-center bg-slate-50 px-4 py-10 text-slate-950">
      <div className="w-full max-w-md">
        <div className="mb-7 text-center">
          <Link
            to="/"
            className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-[8px] bg-slate-950 text-white shadow-xl shadow-slate-950/20"
            aria-label="HealthOps home"
          >
            <Heart className="h-6 w-6" />
          </Link>
          <h1 className="text-2xl font-bold text-slate-950">Sign in to HealthOps</h1>
          <p className="mt-2 text-sm leading-6 text-slate-600">
            Access your protected monitoring workspace.
          </p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="rounded-[8px] border border-slate-200 bg-white p-5 shadow-xl shadow-slate-950/8"
        >
          <div className="mb-5 flex items-start justify-between gap-3">
            <div>
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-950">
                <KeyRound className="h-4 w-4 text-blue-600" />
                Workspace login
              </div>
              <p className="mt-1 text-xs leading-5 text-slate-500">
                Use the admin credentials configured for this deployment.
              </p>
            </div>

            {isDemoMode && (
              <button
                type="button"
                onClick={fillDefaultCredentials}
                className="inline-flex shrink-0 items-center gap-1.5 rounded-[8px] border border-blue-200 bg-blue-50 px-3 py-2 text-xs font-semibold text-blue-700 transition hover:bg-blue-100"
              >
                <Sparkles className="h-3.5 w-3.5" />
                Use default
              </button>
            )}
          </div>

          {error && (
            <div className="mb-4 flex items-center gap-2 rounded-[8px] bg-red-50 px-3 py-2 text-sm text-red-700">
              <AlertCircle className="h-4 w-4 shrink-0" />
              {error}
            </div>
          )}

          <div className="space-y-4">
            <label className="block">
              <span className="mb-1.5 block text-sm font-medium text-slate-700">Username</span>
              <input
                type="text"
                autoComplete="username"
                required
                value={username}
                onChange={e => setUsername(e.target.value)}
                className="h-11 w-full rounded-[8px] border border-slate-300 bg-white px-3 text-sm font-medium text-slate-950 outline-none transition placeholder:text-slate-400 focus:border-blue-500 focus:ring-4 focus:ring-blue-100"
                placeholder="admin"
              />
            </label>

            <label className="block">
              <span className="mb-1.5 block text-sm font-medium text-slate-700">Password</span>
              <input
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={e => setPassword(e.target.value)}
                className="h-11 w-full rounded-[8px] border border-slate-300 bg-white px-3 text-sm font-medium text-slate-950 outline-none transition placeholder:text-slate-400 focus:border-blue-500 focus:ring-4 focus:ring-blue-100"
                placeholder="Password"
              />
            </label>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="mt-5 inline-flex h-11 w-full items-center justify-center gap-2 rounded-[8px] bg-blue-600 px-5 text-sm font-semibold text-white shadow-lg shadow-blue-600/20 transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? 'Signing in...' : 'Sign in'}
            {!loading && <ArrowRight className="h-4 w-4" />}
          </button>
        </form>

        <div className="mt-4 text-center">
          <Link to="/" className="text-sm font-medium text-slate-600 hover:text-slate-950">
            Back to product overview
          </Link>
        </div>
      </div>
    </div>
  )
}
