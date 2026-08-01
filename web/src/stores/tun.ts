import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { tunApi } from '@/api'
import { withLoading } from '@/utils/async'
import type { ConsoleSnapshot, TunStatusResponse } from '@/types'

export const useTunStore = defineStore('tun', () => {
  const status = ref<TunStatusResponse>({ enabled: false, state: 'disabled' })
  const loading = ref(false)

  const isEnabled = computed(() => status.value.enabled)
  const isTransitioning = computed(() => status.value.state === 'transitioning')

  async function fetchStatus() {
    status.value = await tunApi.getStatus()
  }

  async function enable() {
    await withLoading(loading, async () => {
      await tunApi.enable()
      await fetchStatus()
    })
  }

  async function disable() {
    await withLoading(loading, async () => {
      await tunApi.disable()
      await fetchStatus()
    })
  }

  function applyConsoleSnapshot(view: ConsoleSnapshot) {
    status.value = {
      enabled: view.runtime.tun_enabled,
      state: view.runtime.tun_state
    }
  }

  return {
    loading,
    isEnabled, isTransitioning,
    enable, disable, applyConsoleSnapshot
  }
})
