import type { OperationProgress } from './operation'

// 命名约定：Node 等响应类型用 snake_case 匹配 Go JSON wire format；
// NodeFilterBase 等前端内部类型用 camelCase，由 api/node.ts toNodeParams 在 API 边界翻译。

export interface Transport {
  network?: string
  security?: string
  [key: string]: unknown
}

export interface ProtocolOption {
  value: string
  label: string
}

export interface Node {
  id: number
  subscription_id: number
  name: string
  protocol: string
  protocol_label: string
  address: string
  port: number
  raw_url: string
  transport: Transport
  latency: number
  created_at: string
  updated_at: string
}

export interface NodeFilterBase {
  protocol?: string
  subscriptionId?: number
  keyword?: string
  latencyStatuses?: NodeLatencyStatus[]
}

export interface NodeListQuery extends NodeFilterBase {
  cursor?: string
  limit?: number
}

export type NodeLatencyStatus = 'pending' | 'available' | 'timeout'

export interface NodeListResponse {
  items: Node[]
  next_cursor: string
}

export interface NodeSpeedTestStatus {
  running: boolean
  progress?: OperationProgress
  error?: string
}
