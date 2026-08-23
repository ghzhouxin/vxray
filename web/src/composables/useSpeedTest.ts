import { computed, ref } from 'vue'
import { useNodeStore, useOperationStore, useSettingsStore, useXrayStore } from '@/stores'
import { NODE_REFRESH_INTERVAL, SPEED_TEST_STATUS_POLL_INTERVAL, SPEED_TEST_FAILED } from '@/constants'
import { handleError, msg } from '@/utils/message'
import { SSERequestError } from '@/utils/sse'
import { useAutoRefresh } from './useAutoRefresh'
import { useActionExecutor } from './useActionExecutor'
import type { NodeFilterBase, NodeLatencyStatus, NodeSpeedTestStatus, Node, RefreshContext } from '@/types'

export function useSpeedTest(ctx: RefreshContext) {
  const nodeStore = useNodeStore()
  const operationStore = useOperationStore()
  const xrayStore = useXrayStore()
  const settingsStore = useSettingsStore()
  const { refreshConsoleAndNodes } = ctx
  const { execute } = useActionExecutor()

  const autoSpeedTestPending = ref(false)

  const { start: startStatusPolling, stop: stopStatusPolling } = useAutoRefresh(
    () => pollSpeedTestStatus(), SPEED_TEST_STATUS_POLL_INTERVAL,
    { autoStart: false, onError: e => handleError(e, '刷新测速状态失败') }
  )
  const { start: startNodeRefresh, stop: stopNodeRefresh } = useAutoRefresh(
    () => nodeStore.fetchNodes(undefined, true), NODE_REFRESH_INTERVAL,
    { autoStart: false, onError: e => handleError(e, '刷新节点失败') }
  )

  const speedTestTaskStatus = computed(() => {
    const active = operationStore.active
    if (!active || active.status !== 'running') return null
    return {
      title: '节点测速',
      completed: active.completed, total: active.total,
      available: active.success, timeout: active.failed, failed: active.failed
    }
  })

  function restoreRunningJob(status: NodeSpeedTestStatus, message: string) {
    const progress = status.progress || null
    if (progress) operationStore.applyProgress(progress)
    startStatusPolling()
    startNodeRefresh()
    msg.warning(message)
  }

  async function pollSpeedTestStatus() {
    try {
      const status = await nodeStore.fetchSpeedTestStatus()
      const progress = status.progress || null
      if (status.running) { if (progress) operationStore.applyProgress(progress); return }
      stopStatusPolling()
      stopNodeRefresh()
      if (progress && operationStore.type === 'node_speedtest') {
        operationStore.applyProgress(progress)
        operationStore.clear()
        await refreshConsoleAndNodes().catch(e => { console.warn(e); /* 静默刷新，不干扰用户 */ })
      }
    } catch (e) { console.warn(e); /* 静默刷新，不干扰用户 */ }
  }

  async function restoreSpeedTestStatus() {
    const status = await nodeStore.fetchSpeedTestStatus()
    if (status.running) restoreRunningJob(status, '测速任务恢复中')
  }

  function getRunningSpeedTestStatus(error: unknown): NodeSpeedTestStatus | null {
    if (!(error instanceof SSERequestError) || error.status !== 409) return null
    const body = error.data as { data?: NodeSpeedTestStatus } | null
    if (!body?.data) return null
    return body.data
  }

  async function runNodeSpeedTest(payload: { ids?: number[]; filter?: NodeFilterBase }, successMessage?: string) {
    if (operationStore.running) { msg.warning('当前有测速任务执行中'); return }
    stopStatusPolling()
    // 测速期间停掉节点列表全量刷新，延迟由 SSE 事件原地更新，避免整表覆盖竞态
    stopNodeRefresh()
    operationStore.start('node_speedtest')
    let keepRestoredJob = false
    try {
      const lastProgress = await nodeStore.speedTest(payload, progress => operationStore.applyProgress(progress))
      if (lastProgress?.status === 'empty') { msg.warning(lastProgress.message || '当前筛选无可测速节点'); return }
      if (lastProgress?.status === 'failed') throw new Error(lastProgress.error || lastProgress.message || '测速失败')
      await refreshConsoleAndNodes()
      if (successMessage) msg.success(successMessage)
    } catch (error) {
      const runningStatus = getRunningSpeedTestStatus(error)
      if (runningStatus) { keepRestoredJob = true; restoreRunningJob(runningStatus, '已有测速任务执行中'); return }
      await refreshConsoleAndNodes().catch(e => { console.warn(e); /* 静默刷新，不干扰用户 */ })
      const status = await nodeStore.fetchSpeedTestStatus().catch(e => { console.warn(e); return null })
      if (status?.running) { keepRestoredJob = true; restoreRunningJob(status, '测速仍在后台执行'); return }
      handleError(error, SPEED_TEST_FAILED)
    } finally {
      if (!keepRestoredJob) { operationStore.clear(); stopNodeRefresh() }
    }
  }

  function runBatchSpeedTest(latencyStatuses?: NodeLatencyStatus[], successMessage?: string) {
    const filter = latencyStatuses
      ? { ...nodeStore.activeFilter, latencyStatuses }
      : nodeStore.activeFilter
    return runNodeSpeedTest({ filter }, successMessage)
  }

  function handleRetestTimeout() {
    return runBatchSpeedTest(['pending', 'timeout'], '重测完成')
  }

  function handleSpeedTestAvailable() {
    return runBatchSpeedTest(['available'], '测速完成')
  }

  function handleNodeSpeedTest(node: Node) {
    return runNodeSpeedTest({ ids: [node.id] })
  }

  async function triggerAutoSpeedTest() {
    await execute(
      async () => {
        if (!xrayStore.websiteSpeedTestLoading) {
          await xrayStore.runWebsiteSpeedTest()
          await settingsStore.fetchConfigView()
        }
      },
      {
        refreshAfterAction: refreshConsoleAndNodes,
        loading: autoSpeedTestPending,
        errorMsg: '自动测速启动失败'
      }
    )
  }

  function cleanup() {
    stopStatusPolling()
    stopNodeRefresh()
    operationStore.clear()
  }

  return {
    speedTestTaskStatus,
    autoSpeedTestPending,
    handleRetestTimeout, handleSpeedTestAvailable, handleNodeSpeedTest,
    triggerAutoSpeedTest,
    restoreSpeedTestStatus,
    cleanup
  }
}
