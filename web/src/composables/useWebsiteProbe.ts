import { ref } from 'vue'
import { useNodeStore, useXrayStore } from '@/stores'
import { handleError, msg } from '@/utils/message'
import { waitForProxyReady } from '@/utils/async'
import { PROBE_OK_RATIO } from '@/constants'
import type { RefreshContext } from '@/types'

export function useWebsiteProbe(ctx: RefreshContext) {
  const nodeStore = useNodeStore()
  const xrayStore = useXrayStore()
  const { refreshConsoleAndNodes, refreshLogsSilently, showLogs } = ctx

  const probing = ref(false)
  const probeMessage = ref('')

  async function runProbe() {
    if (probing.value) return
    if (!nodeStore.nodes.length) { msg.warning('暂无可用节点'); return }

    probing.value = true
    showLogs()
    try {
      const candidates = nodeStore.nodes
        .filter(n => n.latency > 0)
        .sort((a, b) => a.latency - b.latency)

      if (!candidates.length) { msg.warning('暂无可用节点'); return }

      let bestOkCount = 0
      let bestNodeName = ''

      for (let i = 0; i < candidates.length; i++) {
        const node = candidates[i]
        probeMessage.value = `${i + 1}/${candidates.length}`

        await nodeStore.activateNode(node.id)
        await refreshConsoleAndNodes()
        await refreshLogsSilently()
        await waitForProxyReady()

        const results = await xrayStore.runSpeedTestMulti()
        const okCount = results.filter(r => r.latency > 0).length
        if (okCount > bestOkCount) { bestOkCount = okCount; bestNodeName = node.name }

        if (okCount >= Math.floor(results.length * PROBE_OK_RATIO)) {
          msg.success(`探测完成：${node.name}（${okCount}/${results.length} 网站可用）`)
          await refreshLogsSilently()
          return
        }
      }

      msg.warning(`探测完毕，最佳节点 ${bestNodeName}（${bestOkCount} 网站可用）`)
      await refreshLogsSilently()
    } catch (e) {
      handleError(e, '网站探测失败')
    } finally {
      probing.value = false
      probeMessage.value = ''
    }
  }

  return { probing, probeMessage, runProbe }
}
