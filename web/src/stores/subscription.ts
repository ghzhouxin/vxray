import { defineStore } from 'pinia'
import { ref } from 'vue'
import { subscriptionApi } from '@/api'
import type { ConsoleSnapshot, Subscription, SubscriptionBatchUpdateResult, SubscriptionFormData } from '@/types'

export const useSubscriptionStore = defineStore('subscription', () => {
  const subscriptions = ref<Subscription[]>([])

  function applyConsoleSnapshot(snapshot: ConsoleSnapshot) {
    subscriptions.value = snapshot.subscriptions
  }

  async function addSubscription(data: SubscriptionFormData) {
    const sub = await subscriptionApi.addSubscription(data)
    subscriptions.value = [...subscriptions.value, sub]
    return sub
  }

  async function updateSubscription(id: number, data: Partial<SubscriptionFormData>) {
    const sub = await subscriptionApi.updateSubscription(id, data)
    const index = subscriptions.value.findIndex(s => s.id === id)
    if (index !== -1) subscriptions.value[index] = sub
    return sub
  }

  async function deleteSubscription(id: number) {
    await subscriptionApi.deleteSubscription(id)
    subscriptions.value = subscriptions.value.filter(s => s.id !== id)
  }

  async function refreshSubscriptions(ids?: number[], onProgress?: (progress: any) => void): Promise<SubscriptionBatchUpdateResult | void> {
    if (onProgress) {
      await subscriptionApi.refreshSubscriptions(ids?.length ? { ids } : undefined, onProgress)
      return
    }
    return subscriptionApi.refreshSubscriptions(ids?.length ? { ids } : undefined) as Promise<SubscriptionBatchUpdateResult>
  }

  return { subscriptions, applyConsoleSnapshot, addSubscription, updateSubscription, deleteSubscription, refreshSubscriptions }
})
