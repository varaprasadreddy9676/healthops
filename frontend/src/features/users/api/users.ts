import { api } from '@/shared/api/client'
import type { User, CreateUserRequest, UpdateUserRequest } from "@/shared/types"

export const usersApi = {
  list: () => api.get<User[]>('/users'),
  get: (id: string) => api.get<User>(`/users/${encodeURIComponent(id)}`),
  create: (data: CreateUserRequest) => api.post<User>('/users', data),
  update: (id: string, data: UpdateUserRequest) => api.put<User>(`/users/${encodeURIComponent(id)}`, data),
  delete: (id: string) => api.delete(`/users/${encodeURIComponent(id)}`),
}
