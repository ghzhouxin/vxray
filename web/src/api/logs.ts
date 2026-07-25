import request from '@/utils/request'
import type { LogListResponse } from '@/types'

export const logsApi = {
  getLogs: (params: { level?: string; tag?: string; keyword?: string; cursor?: string; limit?: number }) =>
    request.get<LogListResponse>('/logs', { params }),
  clearLogs: () => request.delete('/logs'),
}
