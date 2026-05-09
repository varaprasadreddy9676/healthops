import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import { api } from "@/shared/api/client"
import type { User } from "@/shared/types"

interface AuthState {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  isAdmin: boolean
  isLoading: boolean
}

interface AuthContextType extends AuthState {
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

const TOKEN_KEY = 'healthops_token'
const USER_KEY = 'healthops_user'

interface LoginResponse {
  token: string
  user: User
}

interface MeResponse {
  user: User
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>(() => {
    const token = localStorage.getItem(TOKEN_KEY)
    const userStr = localStorage.getItem(USER_KEY)
    let user: User | null = null
    try {
      if (userStr) user = JSON.parse(userStr)
    } catch { /* ignore */ }

    return {
      token,
      user,
      isAuthenticated: !!token && !!user,
      isAdmin: user?.role === 'admin',
      isLoading: !!token,
    }
  })

  useEffect(() => {
    const token = state.token
    if (!token) return

    let cancelled = false

    api.get<MeResponse>('/auth/me')
      .then(body => {
        if (cancelled) return
        const user = body.user
        if (user) {
          setState(s => ({
            ...s,
            user,
            isAuthenticated: true,
            isAdmin: user.role === 'admin',
            isLoading: false,
          }))
          localStorage.setItem(USER_KEY, JSON.stringify(user))
        }
      })
      .catch(() => {
        if (cancelled) return
        localStorage.removeItem(TOKEN_KEY)
        localStorage.removeItem(USER_KEY)
        setState({ user: null, token: null, isAuthenticated: false, isAdmin: false, isLoading: false })
      })

    return () => {
      cancelled = true
    }
  }, [state.token])

  const login = useCallback(async (username: string, password: string) => {
    const { token, user } = await api.post<LoginResponse>('/auth/login', { username, password })

    localStorage.setItem(TOKEN_KEY, token)
    localStorage.setItem(USER_KEY, JSON.stringify(user))

    setState({
      token,
      user,
      isAuthenticated: true,
      isAdmin: user.role === 'admin',
      isLoading: false,
    })
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    setState({ user: null, token: null, isAuthenticated: false, isAdmin: false, isLoading: false })
  }, [])

  return (
    <AuthContext.Provider value={{ ...state, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
