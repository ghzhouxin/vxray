import { reactive } from 'vue'
import { useNodeStore, useProxyStore, useSubscriptionStore, useXrayStore } from '@/stores'
import { consoleApi } from '@/api'
import type { ConsoleSnapshot } from '@/types'

export function useConsoleRefresh(options: {
  applyLogsSnapshot: (snapshot: ConsoleSnapshot) => void
  loadLogs: (reset?: boolean) => Promise<void>
}) {
  const xrayStore = useXrayStore()
  const nodeStore = useNodeStore()
  const subscriptionStore = useSubscriptionStore()
  const proxyStore = useProxyStore()
  const nodeSummary = reactive({ all: 0, available: 0, pending: 0, timeout: 0 })
  const runtimePorts = reactive({ http: 0, socks: 0 })

  function applySnapshot(snapshot: ConsoleSnapshot) {
    xrayStore.applyConsoleSnapshot(snapshot)
    xrayStore.applySpeedTestSnapshot(snapshot.speedtest_targets)
    subscriptionStore.setSubscriptions(snapshot.subscriptions)
    nodeStore.setProtocols(snapshot.protocols || [])
    proxyStore.setProxyStatus(snapshot.runtime.proxy.enabled)
    nodeSummary.all = snapshot.node_summary.all
    nodeSummary.available = snapshot.node_summary.available
    nodeSummary.pending = snapshot.node_summary.pending
    nodeSummary.timeout = snapshot.node_summary.timeout
    runtimePorts.http = snapshot.runtime.ports.http || snapshot.runtime.proxy.http_port || 0
    runtimePorts.socks = snapshot.runtime.ports.socks || snapshot.runtime.proxy.socks_port || 0
    options.applyLogsSnapshot(snapshot)
  }

  async function refreshConsole() { applySnapshot(await consoleApi.get()) }
  async function refreshNodes() { await nodeStore.fetchNodes() }
  async function refreshConsoleAndNodes() { await Promise.all([refreshConsole(), refreshNodes()]) }
  /** 静默刷新日志：吞掉错误，用于后台自动刷新场景（不干扰用户） */
  async function refreshLogsSilently() { await options.loadLogs(true).catch(() => {}) }

  return {
    nodeSummary,
    runtimePorts,
    refreshConsole,
    refreshNodes,
    refreshConsoleAndNodes,
    refreshLogsSilently
  }
}
