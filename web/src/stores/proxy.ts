import { defineStore } from 'pinia'
import { ref } from 'vue'
import { proxyApi } from '@/api'
import { withLoading } from '@/utils/async'
import type { ConsoleSnapshot } from '@/types'

export const useProxyStore = defineStore('proxy', () => {
  const systemProxyEnabled = ref(false)
  const loading = ref(false)

  async function toggleProxy() {
    const previous = systemProxyEnabled.value
    systemProxyEnabled.value = !previous
    try {
      await withLoading(loading, () => proxyApi.toggle(systemProxyEnabled.value))
    } catch (e) {
      systemProxyEnabled.value = previous
      throw e
    }
  }

  function applyConsoleSnapshot(snapshot: ConsoleSnapshot) {
    systemProxyEnabled.value = snapshot.runtime.proxy.enabled
  }

  return { systemProxyEnabled, loading, toggleProxy, applyConsoleSnapshot }
})
