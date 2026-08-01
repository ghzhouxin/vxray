import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { xrayApi } from '@/api'
import { withLoading } from '@/utils/async'
import { useSettingsStore } from './settings'
import type { ConsoleSnapshot, XrayStatusResponse } from '@/types'

export const useXrayStore = defineStore('xray', () => {
  const status = ref<XrayStatusResponse>({ running: false })
  const loading = ref(false)

  const isRunning = computed(() => status.value.running)
  const currentNode = computed(() => status.value.current_node)

  async function fetchStatus() {
    status.value = await xrayApi.getRuntime()
  }

  async function startXray() {
    await withLoading(loading, async () => {
      await xrayApi.startRuntime()
      await fetchStatus()
    })
  }

  async function stopXray() {
    await withLoading(loading, async () => {
      await xrayApi.stopRuntime()
      await fetchStatus()
    })
  }

  function applyConsoleSnapshot(view: ConsoleSnapshot) {
    status.value = {
      running: view.runtime.running,
      current_node: view.runtime.current_node
    }
  }

  const speedTesting = ref(false)

  async function runSpeedTestMulti(): Promise<void> {
    await withLoading(speedTesting, async () => {
      await xrayApi.speedTestWebsites()
      await useSettingsStore().fetchConfigView()
    })
  }

  return {
    loading, speedTesting,
    isRunning, currentNode,
    fetchStatus, startXray, stopXray, applyConsoleSnapshot,
    runSpeedTestMulti
  }
})
