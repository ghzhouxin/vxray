type OperationStatus = 'running' | 'success' | 'failed' | 'empty'

export type OperationType = 'node_speedtest' | 'subscription_update'

export interface OperationProgress {
  type: OperationType
  status: OperationStatus
  total: number
  completed: number
  success: number
  failed: number
  node_id?: number
  latency?: number
  error?: string
  message?: string
}
