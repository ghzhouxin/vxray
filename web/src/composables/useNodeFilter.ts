import { computed, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useNodeStore } from '@/stores'
import { handleError } from '@/utils/message'
import { DEBOUNCE_MS } from '@/constants'

export function useNodeFilter(options: {
  refreshNodes: () => Promise<void>
}) {
  const nodeStore = useNodeStore()

  const keyword = computed({ get: () => nodeStore.filter.keyword || '', set: v => { nodeStore.filter.keyword = v || undefined } })
  const protocolFilter = computed({ get: () => nodeStore.filter.protocol || '', set: v => { nodeStore.filter.protocol = v || undefined } })
  const subscriptionFilter = computed({ get: () => nodeStore.filter.subscriptionId || '', set: (v: number | '') => { nodeStore.filter.subscriptionId = v || undefined } })

  const debouncedFetch = useDebounceFn(() => options.refreshNodes().catch(e => handleError(e, '加载节点失败')), DEBOUNCE_MS)
  watch([keyword, protocolFilter, subscriptionFilter], debouncedFetch)

  return { keyword, protocolFilter, subscriptionFilter }
}
