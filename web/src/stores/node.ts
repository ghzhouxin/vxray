import { defineStore } from 'pinia'
import { ref, reactive, computed } from 'vue'
import { nodeApi } from '@/api'
import { LATENCY_TIMEOUT, NODE_PAGE_SIZE } from '@/constants'
import { withLoading } from '@/utils/async'
import type { Node, NodeFilterBase, NodeListQuery, OperationProgress, ProtocolOption } from '@/types'

export const useNodeStore = defineStore('node', () => {
  const nodes = ref<Node[]>([])
  const protocols = ref<ProtocolOption[]>([])
  const loading = ref(false)
  const loadingMore = ref(false)
  const nextCursor = ref('')

  const filter = reactive<NodeListQuery>({})

  const activeFilter = computed<NodeFilterBase>(() => {
    const result: NodeFilterBase = {}
    if (filter.keyword) result.keyword = filter.keyword
    if (filter.subscriptionId) result.subscriptionId = filter.subscriptionId
    if (filter.protocol) result.protocol = filter.protocol
    if (filter.latencyStatuses?.length) result.latencyStatuses = filter.latencyStatuses
    return result
  })

  const hasMore = computed(() => nextCursor.value !== '')

  const updateNodeLatency = (nodeId: number, latency?: number | null) => {
    const index = nodes.value.findIndex(n => n.id === nodeId)
    if (index !== -1) {
      nodes.value.splice(index, 1, { ...nodes.value[index], latency: latency ?? LATENCY_TIMEOUT })
    }
  }

  async function fetchNodes(queryOverride?: NodeListQuery, silent = false) {
    const fn = async () => {
      const result = await nodeApi.getNodes({ ...(queryOverride ?? activeFilter.value), cursor: '', limit: NODE_PAGE_SIZE })
      nodes.value = result.items
      nextCursor.value = result.next_cursor
    }
    if (silent) await fn()
    else await withLoading(loading, fn)
  }

  async function loadMoreNodes() {
    if (!hasMore.value || loadingMore.value || loading.value) return
    await withLoading(loadingMore, async () => {
      const result = await nodeApi.getNodes({ ...activeFilter.value, cursor: nextCursor.value, limit: NODE_PAGE_SIZE })
      nodes.value = [...nodes.value, ...result.items]
      nextCursor.value = result.next_cursor
    })
  }

  async function speedTest(
    payload: { filter?: NodeFilterBase; ids?: number[] },
    onProgress?: (p: OperationProgress) => void
  ): Promise<OperationProgress | null> {
    let lastProgress: OperationProgress | null = null
    await nodeApi.speedTest(payload, progress => {
      lastProgress = progress
      if (progress.node_id != null && progress.testing === false) {
        updateNodeLatency(progress.node_id, progress.latency)
      }
      onProgress?.(progress)
    })
    return lastProgress
  }

  async function deleteNode(id: number) {
    await nodeApi.deleteNode(id)
    nodes.value = nodes.value.filter(n => n.id !== id)
  }

  async function deleteFailedNodes(filterOverride?: NodeFilterBase) {
    const result = await nodeApi.deleteFailedNodes(filterOverride)
    return result.count
  }

  async function activateNode(id: number) {
    await nodeApi.activateNode(id)
  }

  function setProtocols(list: ProtocolOption[]) { protocols.value = list }

  return {
    nodes, protocols, loading, loadingMore, hasMore, filter, activeFilter,
    fetchNodes, loadMoreNodes, speedTest,
    deleteNode, deleteFailedNodes, activateNode,
    setProtocols
  }
})
