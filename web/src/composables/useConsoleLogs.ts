import { reactive, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { logsApi } from '@/api'
import { LOG_DEFAULT_PAGE_SIZE, LOG_REFRESH_INTERVAL, DEBOUNCE_MS } from '@/constants'
import { handleError, msg } from '@/utils/message'
import { useAutoRefresh } from './useAutoRefresh'
import type { ConsoleSnapshot, Log } from '@/types'

const REFRESH_LOGS_ERROR = '刷新日志失败'

export function useConsoleLogs(getSafeEls?: () => HTMLElement[]) {
  const collapsed = ref(true)
  const autoRefresh = ref(true)
  const logs = ref<Log[]>([])
  const tags = ref<string[]>([])
  const levels = ref<string[]>([])
  const loading = ref(false)
  const nextCursor = ref('')
  const hasMore = ref(false)
  const filter = reactive({ level: '', tag: '', keyword: '' })

  const { start: startRefresh, stop: stopRefresh, isRunning: refreshRunning } = useAutoRefresh(
    () => loadLogs(true), LOG_REFRESH_INTERVAL, { autoStart: false, onError: e => handleError(e, REFRESH_LOGS_ERROR) }
  )

  function syncRefresh() {
    if (collapsed.value || !autoRefresh.value) {
      stopRefresh()
      return
    }
    if (!refreshRunning.value) startRefresh()
  }

  const debouncedLoadLogs = useDebounceFn(() => {
    if (!collapsed.value) loadLogs(true).catch(e => handleError(e, REFRESH_LOGS_ERROR))
  }, DEBOUNCE_MS)

  watch(filter, debouncedLoadLogs, { deep: true })

  watch(collapsed, value => {
    if (value) { stopRefresh(); return }
    autoRefresh.value = true
    loadLogs(true)
      .then(() => syncRefresh())
      .catch(e => handleError(e, REFRESH_LOGS_ERROR))
  })

  watch(autoRefresh, () => syncRefresh())

  function applySnapshot(snapshot: ConsoleSnapshot) {
    logs.value = snapshot.logs.items
    tags.value = snapshot.logs.tags
    levels.value = snapshot.logs.levels
    nextCursor.value = snapshot.logs.next_cursor
    hasMore.value = snapshot.logs.has_more
  }

  async function loadLogs(reset = true) {
    loading.value = true
    try {
      const result = await logsApi.getLogs({
        ...filter,
        cursor: reset ? undefined : nextCursor.value || undefined,
        limit: LOG_DEFAULT_PAGE_SIZE
      })
      logs.value = reset ? result.items : [...logs.value, ...result.items]
      tags.value = result.tags
      levels.value = result.levels
      nextCursor.value = result.next_cursor
      hasMore.value = result.has_more
    } finally {
      loading.value = false
    }
  }

  async function loadMore() { if (hasMore.value && !loading.value) await loadLogs(false) }

  async function clearLogs() {
    try {
      await logsApi.clearLogs()
      logs.value = []
      tags.value = []
      levels.value = []
      nextCursor.value = ''
      hasMore.value = false
      filter.level = ''
      filter.tag = ''
      filter.keyword = ''
      msg.success('日志已清空')
    } catch (e) {
      handleError(e, '清空日志失败')
    }
  }

  function showLogs() { collapsed.value = false }

  function handleConsoleClick(event: MouseEvent) {
    if (collapsed.value) return
    const target = event.target as HTMLElement | null
    if (target && !getSafeEls?.().some(el => el.contains(target))) collapsed.value = true
  }

  return {
    collapsed,
    autoRefresh,
    logs,
    tags,
    levels,
    loading,
    hasMore,
    filter,
    applySnapshot,
    loadLogs,
    loadMore,
    clearLogs,
    showLogs,
    handleConsoleClick
  }
}
