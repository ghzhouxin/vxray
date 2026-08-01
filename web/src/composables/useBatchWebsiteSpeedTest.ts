import { computed, ref } from 'vue'
import { useNodeStore, useSettingsStore, useXrayStore } from '@/stores'
import { handleError, msg } from '@/utils/message'
import { waitForProxyReady } from '@/utils/async'
import { NO_AVAILABLE_NODE, BATCH_EARLY_STOP_RATIO } from '@/constants'
import type { RefreshContext } from '@/types'

// useBatchWebsiteSpeedTest 实现前端轮流网站测速：按延迟升序遍历可用节点，逐个切换并触发网站测速，
// ok 比率达到 BATCH_EARLY_STOP_RATIO 即早退。整个过程在前端循环，无后端 job。
export function useBatchWebsiteSpeedTest(ctx: RefreshContext) {
  const nodeStore = useNodeStore()
  const xrayStore = useXrayStore()
  const settingsStore = useSettingsStore()
  const { refreshConsoleAndNodes, refreshLogsSilently, showLogs } = ctx

  const batchLoading = ref(false)
  const batchCurrent = ref(0)
  const batchTotal = ref(0)

  const batchProgress = computed(() => {
    if (!batchLoading.value) return ''
    return `${batchCurrent.value}/${batchTotal.value}`
  })

  function countOk(): number {
    return settingsStore.settings.speedtest.website_targets.filter(t => t.latency > 0).length
  }

  async function runBatch() {
    if (batchLoading.value) return
    const candidates = nodeStore.nodes
      .filter(n => n.latency > 0)
      .slice()
      .sort((a, b) => (a.latency ?? 0) - (b.latency ?? 0))
    if (!candidates.length) { msg.warning(NO_AVAILABLE_NODE); return }

    showLogs()
    batchLoading.value = true
    batchTotal.value = candidates.length
    batchCurrent.value = 0

    let bestOk = 0
    const total = settingsStore.settings.speedtest.website_targets.length
    try {
      for (let i = 0; i < candidates.length; i++) {
        batchCurrent.value = i + 1
        try {
          await nodeStore.activateNode(candidates[i].id)
          await waitForProxyReady()
          await xrayStore.runWebsiteSpeedTest()
          await settingsStore.fetchConfigView()
          const ok = countOk()
          if (ok > bestOk) bestOk = ok
          await refreshConsoleAndNodes().catch(e => console.warn(e))
          await refreshLogsSilently()
          if (total > 0 && ok >= total * BATCH_EARLY_STOP_RATIO) break
        } catch (e) {
          console.warn('batch website speed test node failed', candidates[i].id, e)
        }
      }
      if (bestOk === 0) msg.warning(`探测完毕，无可用节点（0/${total}）`)
      else if (bestOk >= total) msg.success(`探测完成（${bestOk}/${total} 网站可用）`)
      else msg.warning(`探测完毕（${bestOk}/${total} 网站可用）`)
    } catch (e) {
      handleError(e, '网站探测失败')
    } finally {
      batchLoading.value = false
    }
  }

  return { batchLoading, batchProgress, runBatch }
}
