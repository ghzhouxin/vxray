import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { xrayApi } from '@/api'
import { withLoading } from '@/utils/async'
import type { ConsoleSnapshot, XrayStatusResponse, WebsiteSpeedTestResult } from '@/types'

export const useXrayStore = defineStore('xray', () => {
  // --- Runtime status ---

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

  // --- Website speed test ---

  const speedTesting = ref(false)
  const speedTestResults = ref<WebsiteSpeedTestResult[]>([])

  function normalizeResult(t: WebsiteSpeedTestResult, opts: { resetLatency?: boolean } = {}): WebsiteSpeedTestResult {
    return {
      ...t,
      latency: opts.resetLatency ? 0 : (t.latency || 0),
      error: t.error || ''
    }
  }

  function applySpeedTestSnapshot(targets: WebsiteSpeedTestResult[]) {
    speedTestResults.value = targets.map(t => normalizeResult(t))
  }

  function resetSpeedTestResults() {
    speedTestResults.value = speedTestResults.value.map(t => normalizeResult(t, { resetLatency: true }))
  }

  async function runSpeedTestMulti() {
    await withLoading(speedTesting, async () => {
      resetSpeedTestResults()
      const results = await xrayApi.speedTestWebsites()
      if (results && Array.isArray(results)) {
        speedTestResults.value = results.map(t => normalizeResult(t))
      }
    })
    return speedTestResults.value
  }

  return {
    loading, speedTesting,
    isRunning, currentNode,
    fetchStatus, startXray, stopXray, applyConsoleSnapshot,
    speedTestResults, applySpeedTestSnapshot, runSpeedTestMulti
  }
})
