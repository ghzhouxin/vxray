import type { OperationProgress } from './operation'

export interface StreamSettings {
  network?: string
  security?: string
  tlsSettings?: Record<string, unknown>
  realitySettings?: Record<string, unknown>
  wsSettings?: Record<string, unknown>
  grpcSettings?: Record<string, unknown>
  [key: string]: unknown
}

export interface OutboundConfig {
  streamSettings?: StreamSettings
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
  outbound_config: OutboundConfig
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
}
