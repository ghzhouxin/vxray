import { computed, ref } from 'vue'
import { nodeApi } from '@/api'
import { useNodeStore, useSettingsStore, useXrayStore } from '@/stores'
import { handleError, msg } from '@/utils/message'
import { NO_AVAILABLE_NODE, NODE_PAGE_SIZE } from '@/constants'
import type { Node, RefreshContext } from '@/types'

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
  const batchCancel = ref(false)

  const batchProgress = computed(() => {
    if (!batchLoading.value) return ''
    return `${batchCurrent.value}/${batchTotal.value}`
  })

  function countOk(): number {
    return settingsStore.settings.speedtest.website_targets.filter(t => t.latency > 0).length
  }

  // 候选取第一页可用节点即可：后端已按延迟升序返回，靠前的就是最优候选
  async function collectCandidates(): Promise<Node[]> {
    const resp = await nodeApi.getNodes({ latencyStatuses: ['available'], limit: NODE_PAGE_SIZE })
    return resp.items
  }

  async function runBatch() {
    if (batchLoading.value) return
    const candidates = await collectCandidates().catch(e => {
      handleError(e, '获取可用节点失败')
      return []
    })
    if (!candidates.length) { msg.warning(NO_AVAILABLE_NODE); return }

    const total = settingsStore.settings.speedtest.website_targets.length
    const threshold = Math.max(1, Math.floor(total * 3 / 4))
    const originalNodeId = xrayStore.currentNode?.id

    batchLoading.value = true
    batchCancel.value = false
    batchTotal.value = candidates.length
    batchCurrent.value = 0

    let foundOk = 0
    try {
      for (let i = 0; i < candidates.length; i++) {
        if (batchCancel.value) break
        batchCurrent.value = i + 1
        try {
          await nodeStore.activateNode(candidates[i].id)
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
      } else if (batchCancel.value) {
        if (originalNodeId) {
          await nodeStore.activateNode(originalNodeId).catch(e => console.warn(e))
          await refreshConsoleAndNodes().catch(e => console.warn(e))
        }
        msg.warning('轮流测速已取消，已恢复原节点')
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

  function cancelBatch() {
    if (batchLoading.value) batchCancel.value = true
  }

  return { batchLoading, batchProgress, batchCancel, runBatch, cancelBatch }
}
