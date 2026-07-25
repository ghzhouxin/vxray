import { computed, ref } from 'vue'
import { useNodeStore, useOperationStore, useXrayStore } from '@/stores'
import { nodeApi } from '@/api'
import { NODE_REFRESH_INTERVAL, SPEED_TEST_STATUS_POLL_INTERVAL, SPEED_TEST_FAILED } from '@/constants'
import { handleError, msg } from '@/utils/message'
import { SSERequestError } from '@/utils/sse'
import { waitForProxyReady, withLoading } from '@/utils/async'
import { useAutoRefresh } from './useAutoRefresh'
import type { NodeFilterBase, NodeLatencyStatus, NodeSpeedTestStatus, Node, RefreshContext } from '@/types'

export function useSpeedTest(ctx: RefreshContext) {
  const nodeStore = useNodeStore()
  const operationStore = useOperationStore()
  const xrayStore = useXrayStore()
  const { refreshConsoleAndNodes, refreshLogsSilently, showLogs } = ctx

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
      available: active.success, timeout: active.failed
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
      const status = await nodeApi.getSpeedTestStatus()
      const progress = status.progress || null
      if (status.running) { if (progress) operationStore.applyProgress(progress); return }
      stopStatusPolling()
      stopNodeRefresh()
      if (progress && operationStore.type === 'node_speedtest') {
        operationStore.applyProgress(progress)
        operationStore.clear()
        await refreshConsoleAndNodes().catch(e => { console.warn(e); /* 静默刷新，不干扰用户 */ })
        await refreshLogsSilently()
      }
    } catch (e) { console.warn(e); /* 静默刷新，不干扰用户 */ }
  }

  async function restoreSpeedTestStatus() {
    const status = await nodeApi.getSpeedTestStatus()
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
    stopNodeRefresh()
    startNodeRefresh()
    showLogs()
    operationStore.start('node_speedtest')
    let keepRestoredJob = false
    try {
      const lastProgress = await nodeStore.speedTest(payload, progress => operationStore.applyProgress(progress))
      if (lastProgress?.status === 'empty') { await refreshLogsSilently(); msg.warning(lastProgress.message || '当前筛选无可测速节点'); return }
      if (lastProgress?.status === 'failed') throw new Error(lastProgress.error || lastProgress.message || '测速失败')
      await refreshConsoleAndNodes(); await refreshLogsSilently()
      if (successMessage) msg.success(successMessage)
    } catch (error) {
      const runningStatus = getRunningSpeedTestStatus(error)
      if (runningStatus) { keepRestoredJob = true; restoreRunningJob(runningStatus, '已有测速任务执行中'); return }
      await refreshConsoleAndNodes().catch(e => { console.warn(e); /* 静默刷新，不干扰用户 */ }); await refreshLogsSilently()
      const status = await nodeApi.getSpeedTestStatus().catch(e => { console.warn(e); return null })
      if (status?.running) { keepRestoredJob = true; restoreRunningJob(status, '测速仍在后台执行'); return }
      handleError(error, SPEED_TEST_FAILED)
    } finally {
      if (!keepRestoredJob) { operationStore.clear(); stopNodeRefresh() }
    }
  }

  function runBatchSpeedTest(latencyStatuses?: NodeLatencyStatus[], successMessage?: string) {
    if (latencyStatuses) {
      nodeStore.filter.latencyStatuses = latencyStatuses
    }
    return runNodeSpeedTest({ filter: nodeStore.activeFilter }, successMessage)
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
    try {
      await withLoading(autoSpeedTestPending, async () => {
        if (!xrayStore.speedTesting) {
          await waitForProxyReady()
          await xrayStore.runSpeedTestMulti()
        }
      })
    } catch (e) { handleError(e, '自动测速启动失败') }
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
