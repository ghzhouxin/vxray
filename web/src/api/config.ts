import request from '@/utils/request'
import type { SystemMeta, UserSettings } from '@/types'

export const settingsApi = {
  get: () => request.get<{ settings: UserSettings; system: SystemMeta }>('/settings'),
  getDefault: () => request.get<UserSettings>('/settings/default'),
  update: (settings: UserSettings) => request.put('/settings', settings)
}
