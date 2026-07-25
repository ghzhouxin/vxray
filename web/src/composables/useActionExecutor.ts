import { handleError, msg } from '@/utils/message'
import type { RefreshContext } from '@/types'

interface ActionOptions {
  refreshAfterAction: () => Promise<void>
  showLogsBefore?: boolean
  skipLogsRefresh?: boolean
  successMsg?: string
  errorMsg?: string
}

export function useActionExecutor(ctx: RefreshContext) {
  const { refreshLogsSilently, showLogs } = ctx

  async function execute(
    action: () => Promise<unknown>,
    options: ActionOptions
  ): Promise<boolean> {
    if (options.showLogsBefore) showLogs()
    try {
      await action()
      await options.refreshAfterAction()
      if (options.successMsg) msg.success(options.successMsg)
      return true
    } catch (e) {
      handleError(e, options.errorMsg || '操作失败')
      return false
    } finally {
      if (!options.skipLogsRefresh) await refreshLogsSilently()
    }
  }

  return { execute }
}
