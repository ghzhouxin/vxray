import { computed, ref } from 'vue'
import { useNodeStore, useSettingsStore, useXrayStore } from '@/stores'
import { handleError, msg } from '@/utils/message'
import { NO_AVAILABLE_NODE, SPEED_TEST_FAILED } from '@/constants'
import { useActionExecutor } from './useActionExecutor'
import type { RefreshContext } from '@/types'

// useWebsiteSpeedTest 收拢网站测速全部前端逻辑：
// - runWebsiteSpeedTest：单次测当前活动节点（xray 未运行时自动激活节点）；
// - runBatchWebsiteSpeedTest：批量轮流测速，按延迟升序遍历可用节点，
//   网站通过数达到 floor(total*3/4)（至少1）即停止；未达标恢复原节点。
export function useWebsiteSpeedTest(ctx: RefreshContext) {
  const nodeStore = useNodeStore()
  const xrayStore = useXrayStore()
  const settingsStore = useSettingsStore()
  const { refreshConsole, refreshConsoleAndNodes } = ctx
  const { execute } = useActionExecutor()

  const batchLoading = ref(false)
  const batchCurrent = ref(0)
  const batchTotal = ref(0)
  let cancelled = false

  const batchProgress = computed(() => {
    if (!batchLoading.value) return ''
    return `${batchCurrent.value}/${batchTotal.value}`
  })

  function passedTargetCount(): number {
    return settingsStore.settings.speedtest.website_targets.filter(t => t.latency > 0).length
  }

  // 共享核心：测当前活动节点的全部网站目标，返回通过数
  async function testActiveNodeWebsites(): Promise<number> {
    await xrayStore.speedTestWebsites()
    await settingsStore.fetchConfigView()
    return passedTargetCount()
  }

  async function runWebsiteSpeedTest() {
    if (xrayStore.websiteSpeedTestLoading) { msg.warning('网站测速进行中'); return }
    if (!xrayStore.isRunning || !xrayStore.currentNode) {
      const current = xrayStore.currentNode
      const candidate = (current && nodeStore.nodes.find(n => n.id === current.id))
        ?? nodeStore.nodes.find(n => n.latency > 0)
      if (!candidate) { msg.warning(NO_AVAILABLE_NODE); return }
      await execute(() => nodeStore.activateNode(candidate.id), {
        refreshAfterAction: refreshConsoleAndNodes,
        errorMsg: '激活节点失败'
      })
    }
    await execute(
      () => testActiveNodeWebsites(),
      { refreshAfterAction: refreshConsole, successMsg: '网站测速完成', errorMsg: SPEED_TEST_FAILED }
    )
  }

  // 批量候选：已加载的可用节点按延迟升序显式排序——
  // store 列表顺序只在 fetch 时正确，节点测速后 SSE 原地更新延迟不重排，顺序不可靠
  function collectCandidates() {
    return nodeStore.nodes
      .filter(n => n.latency > 0)
      .sort((a, b) => a.latency - b.latency)
  }

  async function restoreOriginalNode(nodeId: number) {
    await nodeStore.activateNode(nodeId).catch(e => console.warn(e))
    await refreshConsoleAndNodes().catch(e => console.warn(e))
  }

  async function runBatchWebsiteSpeedTest() {
    if (batchLoading.value) return
    const candidates = collectCandidates()
    if (!candidates.length) { msg.warning(NO_AVAILABLE_NODE); return }

    const total = settingsStore.settings.speedtest.website_targets.length
    const threshold = Math.max(1, Math.floor(total * 3 / 4))
    const originalNodeId = xrayStore.currentNode?.id

    batchLoading.value = true
    cancelled = false
    batchTotal.value = candidates.length
    batchCurrent.value = 0

    let foundOk = 0
    try {
      for (const [i, node] of candidates.entries()) {
        if (cancelled) break
        batchCurrent.value = i + 1
        try {
          await nodeStore.activateNode(node.id)
          const ok = await testActiveNodeWebsites()
          if (ok >= threshold) {
            foundOk = ok
            await refreshConsoleAndNodes().catch(e => console.warn(e))
            break
          }
        } catch (e) {
          // 单节点失败不中断，继续下一个
          console.warn('batch website speed test node failed', node.id, e)
        }
      }

      if (foundOk > 0) {
        msg.success(`找到优秀节点（${foundOk}/${total} 网站可用）`)
      } else {
        if (originalNodeId) await restoreOriginalNode(originalNodeId)
        msg.warning(cancelled
          ? '轮流测速已取消，已恢复原节点'
          : `未找到优秀节点（阈值 ${threshold}/${total}）`)
      }
    } catch (e) {
      handleError(e, '网站探测失败')
    } finally {
      batchLoading.value = false
    }
  }

  function cancelBatchWebsiteSpeedTest() {
    if (batchLoading.value) cancelled = true
  }

  return {
    batchWebsiteSpeedTestLoading: batchLoading,
    batchWebsiteSpeedTestProgress: batchProgress,
    runWebsiteSpeedTest,
    runBatchWebsiteSpeedTest,
    cancelBatchWebsiteSpeedTest
  }
}
