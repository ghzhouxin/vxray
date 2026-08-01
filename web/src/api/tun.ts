import request from '@/utils/request'
import type { TunStatusResponse } from '@/types'

const BASE = '/tun'

export const tunApi = {
  getStatus: () => request.get<TunStatusResponse>(`${BASE}/status`),
  enable: () => request.post(`${BASE}/enable`),
  disable: () => request.post(`${BASE}/disable`)
}
