import { type Ref } from 'vue'
import { handleError, msg } from '@/utils/message'
import { withLoading } from '@/utils/async'
import type { RefreshContext } from '@/types'

interface ActionOptions {
  refreshAfterAction: () => Promise<void>
  loading?: Ref<boolean>
  showLogsBefore?: boolean
  skipLogsRefresh?: boolean
  successMsg?: string | (() => string)
  errorMsg?: string
}

export function useActionExecutor(ctx: RefreshContext) {
  const { refreshLogsSilently, showLogs } = ctx

  async function execute(
    action: () => Promise<unknown>,
    options: ActionOptions
  ): Promise<boolean> {
    if (options.showLogsBefore) showLogs()
    const run = async () => {
      try {
        await action()
        await options.refreshAfterAction()
        const text = typeof options.successMsg === 'function' ? options.successMsg() : options.successMsg
        if (text) msg.success(text)
        return true
      } catch (e) {
        handleError(e, options.errorMsg || '操作失败')
        return false
      } finally {
        if (!options.skipLogsRefresh) await refreshLogsSilently()
      }
    }
    return options.loading ? withLoading(options.loading, run) : run()
  }

  return { execute }
}
