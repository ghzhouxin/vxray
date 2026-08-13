import { computed, ref } from 'vue'
import { useNodeStore, useSettingsStore, useXrayStore } from '@/stores'
import { handleError, msg } from '@/utils/message'
import { waitForProxyReady } from '@/utils/async'
import { NO_AVAILABLE_NODE } from '@/constants'
import type { RefreshContext } from '@/types'

// useBatchWebsiteSpeedTest 实现前端轮流网站测速：按延迟升序遍历可用节点，逐个切换并触发网站测速，
// 网站通过数达到 floor(total*3/4)（至少1）即停止；未找到则恢复测速前节点。整个过程在前端循环，无后端 job。
export function useBatchWebsiteSpeedTest(ctx: RefreshContext) {
  const nodeStore = useNodeStore()
  const xrayStore = useXrayStore()
  const settingsStore = useSettingsStore()
  const { refreshConsoleAndNodes } = ctx

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

    const total = settingsStore.settings.speedtest.website_targets.length
    const threshold = Math.max(1, Math.floor(total * 3 / 4))
    const originalNodeId = xrayStore.currentNode?.id

    batchLoading.value = true
    batchTotal.value = candidates.length
    batchCurrent.value = 0

    let foundOk = 0
    try {
      for (let i = 0; i < candidates.length; i++) {
        batchCurrent.value = i + 1
        try {
          await nodeStore.activateNode(candidates[i].id)
          await waitForProxyReady()
          await xrayStore.runWebsiteSpeedTest()
          await settingsStore.fetchConfigView()
          const ok = countOk()
          if (ok >= threshold) {
            foundOk = ok
            await refreshConsoleAndNodes().catch(e => console.warn(e))
            break
          }
          // 未达标，快速跳过，不刷新直接试下一节点
        } catch (e) {
          console.warn('batch website speed test node failed', candidates[i].id, e)
        }
      }
      if (foundOk > 0) {
        msg.success(`找到优秀节点（${foundOk}/${total} 网站可用）`)
      } else {
        // 未找到优秀节点，恢复测速前的节点
        if (originalNodeId) {
          await nodeStore.activateNode(originalNodeId).catch(e => console.warn(e))
          await refreshConsoleAndNodes().catch(e => console.warn(e))
        }
        msg.warning(`未找到优秀节点（阈值 ${threshold}/${total}）`)
      }
    } catch (e) {
      handleError(e, '网站探测失败')
    } finally {
      batchLoading.value = false
    }
  }

  return { batchLoading, batchProgress, runBatch }
}
