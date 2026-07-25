import request from '@/utils/request'
import type { XrayStatusResponse, WebsiteSpeedTestResult } from '@/types'

const BASE = '/xray'

export const xrayApi = {
  getRuntime: () => request.get<XrayStatusResponse>(`${BASE}/runtime`),
  startRuntime: () => request.post(`${BASE}/runtime/start`),
  stopRuntime: () => request.post(`${BASE}/runtime/stop`),
  getConfig: () => request.get<{ content: string }>(`${BASE}/config`),
  saveConfig: (content: string) => request.put(`${BASE}/config`, { content }),
  getDefaultConfig: () => request.get<{ content: string }>(`${BASE}/config/default`),
  speedTestWebsites: () => request.post<WebsiteSpeedTestResult[]>(`${BASE}/speedtest/websites`)
}
