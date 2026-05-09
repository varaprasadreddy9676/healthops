import { api } from '@/shared/api/client'

export interface NotificationChannel {
  id: string
  name: string
  type: string
  enabled: boolean
  webhookUrl?: string
  email?: string
  smtpHost?: string
  smtpPort?: number
  smtpUser?: string
  smtpPass?: string
  fromEmail?: string
  botToken?: string
  chatId?: string
  routingKey?: string
  severities?: string[]
  checkIds?: string[]
  checkTypes?: string[]
  servers?: string[]
  tags?: string[]
  cooldownMinutes?: number
  minConsecutiveFailures?: number
  notifyOnResolve?: boolean
  headers?: Record<string, string>
  createdAt?: string
  updatedAt?: string
}

export const notificationsApi = {
  list: () => api.get<NotificationChannel[]>('/notification-channels'),
  create: (channel: Partial<NotificationChannel>) =>
    api.post<NotificationChannel>('/notification-channels', channel),
  update: (id: string, channel: Partial<NotificationChannel>) =>
    api.put<NotificationChannel>(`/notification-channels/${encodeURIComponent(id)}`, channel),
  delete: (id: string) =>
    api.delete<{ deleted: string }>(`/notification-channels/${encodeURIComponent(id)}`),
  toggle: (id: string, enabled: boolean) =>
    api.post<NotificationChannel>(`/notification-channels/${encodeURIComponent(id)}/toggle`, { enabled }),
  test: (channelId: string) =>
    api.post<{ status: string; message: string }>('/notification-channels/test', { channelId }),
}
