export interface Subscription {
  id: number
  name: string
  url: string
  node_count: number
  created_at: string
  updated_at: string
  last_sync_at?: string
  last_sync_status?: string
}

export interface SubscriptionFormData {
  name: string
  url: string
}

export interface SubscriptionBatchUpdateResult {
  total: number
  success: number
  failed: number
}
