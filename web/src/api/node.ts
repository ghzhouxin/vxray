import request from '@/utils/request'
import { sseRequest } from '@/utils/sse'
import type { NodeFilterBase, NodeListResponse, NodeListQuery, NodeSpeedTestStatus, OperationProgress } from '@/types'

const BASE = '/nodes'

const toNodeParams = (filter?: NodeListQuery) => ({
  subscription_id: filter?.subscriptionId,
  protocol: filter?.protocol,
  keyword: filter?.keyword,
  latency_statuses: filter?.latencyStatuses,
  cursor: filter?.cursor,
  limit: filter?.limit
})

const toSpeedTestPayload = (filter?: NodeFilterBase, ids?: number[]) => {
  if (ids?.length) return { ids }
  return { filter: toNodeParams(filter) }
}

export const nodeApi = {
  getNodes: (filter?: NodeListQuery) => request.get<NodeListResponse>(BASE, { params: toNodeParams(filter) }),
  getSpeedTestStatus: () => request.get<NodeSpeedTestStatus>(`${BASE}/speedtest/status`),
  speedTest: (payload: { filter?: NodeFilterBase; ids?: number[] }, onProgress: (p: OperationProgress) => void) =>
    sseRequest<OperationProgress>(`${BASE}/speedtest`, toSpeedTestPayload(payload.filter, payload.ids), onProgress),
  activateNode: (id: number) => request.post(`${BASE}/${id}/activate`),
  deleteNode: (id: number) => request.delete(`${BASE}/${id}`),
  deleteFailedNodes: (filter?: NodeFilterBase) =>
    request.post<{ count: number }>(`${BASE}/delete-failed`, filter ? toNodeParams(filter) : {})
}
