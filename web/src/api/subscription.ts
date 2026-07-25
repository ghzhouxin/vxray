import request from '@/utils/request'
import type { Subscription, SubscriptionBatchUpdateResult, SubscriptionFormData } from '@/types'

const BASE = '/subscriptions'

export const subscriptionApi = {
  addSubscription: (data: SubscriptionFormData) => request.post<Subscription>(BASE, data),
  updateSubscription: (id: number, data: Partial<SubscriptionFormData>) => request.put<Subscription>(`${BASE}/${id}`, data),
  deleteSubscription: (id: number) => request.delete(`${BASE}/${id}`),
  refreshSubscriptions: (payload?: { ids?: number[] }) =>
    request.post<SubscriptionBatchUpdateResult>(`${BASE}/refresh`, payload ?? {})
}
