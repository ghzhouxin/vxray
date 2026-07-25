import { defineStore } from 'pinia'
import { ref } from 'vue'
import { subscriptionApi } from '@/api'
import type { Subscription, SubscriptionBatchUpdateResult, SubscriptionFormData } from '@/types'

export const useSubscriptionStore = defineStore('subscription', () => {
  const subscriptions = ref<Subscription[]>([])

  function setSubscriptions(next: Subscription[]) {
    subscriptions.value = next || []
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

  async function refreshSubscriptions(ids?: number[]): Promise<SubscriptionBatchUpdateResult> {
    return subscriptionApi.refreshSubscriptions(ids?.length ? { ids } : undefined)
  }

  return { subscriptions, setSubscriptions, addSubscription, updateSubscription, deleteSubscription, refreshSubscriptions }
})
