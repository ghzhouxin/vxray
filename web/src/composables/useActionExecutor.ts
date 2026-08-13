import { type Ref } from 'vue'
import { handleError, msg } from '@/utils/message'
import { withLoading } from '@/utils/async'

interface ActionOptions {
  refreshAfterAction: () => Promise<void>
  loading?: Ref<boolean>
  successMsg?: string | (() => string)
  errorMsg?: string
}

export function useActionExecutor() {
  async function execute(
    action: () => Promise<unknown>,
    options: ActionOptions
  ): Promise<boolean> {
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
      }
    }
    return options.loading ? withLoading(options.loading, run) : run()
  }

  return { execute }
}
