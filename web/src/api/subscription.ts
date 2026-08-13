import request from '@/utils/request'
import { sseRequest } from '@/utils/sse'
import type { OperationProgress, Subscription, SubscriptionBatchUpdateResult, SubscriptionFormData } from '@/types'

const BASE = '/subscriptions'

export const subscriptionApi = {
  addSubscription: (data: SubscriptionFormData) => request.post<Subscription>(BASE, data),
  updateSubscription: (id: number, data: Partial<SubscriptionFormData>) => request.put<Subscription>(`${BASE}/${id}`, data),
  deleteSubscription: (id: number) => request.delete(`${BASE}/${id}`),
  refreshSubscriptions: (payload?: { ids?: number[] }, onProgress?: (p: OperationProgress) => void) => {
    if (onProgress) {
      return sseRequest<OperationProgress>(`${BASE}/refresh`, payload ?? {}, onProgress)
    }
    return request.post<SubscriptionBatchUpdateResult>(`${BASE}/refresh`, payload ?? {})
  }
}
