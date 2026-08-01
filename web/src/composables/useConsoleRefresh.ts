import { reactive } from 'vue'
import { useNodeStore, useProxyStore, useSubscriptionStore, useTunStore, useXrayStore } from '@/stores'
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
  const tunStore = useTunStore()
  const nodeSummary = reactive({ all: 0, available: 0, pending: 0, timeout: 0 })
  const runtimePorts = reactive({ http: 0, socks: 0 })

  function applySnapshot(snapshot: ConsoleSnapshot) {
    xrayStore.applyConsoleSnapshot(snapshot)
    tunStore.applyConsoleSnapshot(snapshot)
    subscriptionStore.applyConsoleSnapshot(snapshot)
    nodeStore.applyConsoleSnapshot(snapshot)
    proxyStore.applyConsoleSnapshot(snapshot)
    nodeSummary.all = snapshot.node_summary.all
    nodeSummary.available = snapshot.node_summary.available
    nodeSummary.pending = snapshot.node_summary.pending
    nodeSummary.timeout = snapshot.node_summary.timeout
    runtimePorts.http = snapshot.runtime.proxy.http_port
    runtimePorts.socks = snapshot.runtime.proxy.socks_port
    options.applyLogsSnapshot(snapshot)
  }

  async function refreshConsole() { applySnapshot(await consoleApi.get()) }
  async function refreshNodes() { await nodeStore.fetchNodes() }
  async function refreshConsoleAndNodes() { await Promise.all([refreshConsole(), refreshNodes()]) }
  async function refreshLogsSilently() { await options.loadLogs(true).catch(e => console.warn(e)) }

  return {
    nodeSummary,
    runtimePorts,
    refreshConsole,
    refreshNodes,
    refreshConsoleAndNodes,
    refreshLogsSilently
  }
}
