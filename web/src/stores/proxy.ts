import { defineStore } from 'pinia'
import { ref } from 'vue'
import { proxyApi } from '@/api'
import { withLoading } from '@/utils/async'

export const useProxyStore = defineStore('proxy', () => {
  const systemProxyEnabled = ref(false)
  const proxyLoading = ref(false)

  async function toggleProxy() {
    const previous = systemProxyEnabled.value
    systemProxyEnabled.value = !previous
    try {
      await withLoading(proxyLoading, () => proxyApi.toggle(systemProxyEnabled.value))
    } catch (e) {
      systemProxyEnabled.value = previous
      throw e
    }
  }

  function setProxyStatus(enabled: boolean) {
    systemProxyEnabled.value = enabled
  }

  return { systemProxyEnabled, proxyLoading, toggleProxy, setProxyStatus }
})
