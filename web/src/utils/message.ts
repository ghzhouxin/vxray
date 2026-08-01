import { AxiosError } from 'axios'
import { MESSAGE_DURATION } from '@/constants'
import { ElMessage } from 'element-plus'

let pendingError = ''

function showMessage(type: 'success' | 'error' | 'warning', message: string, duration: number) {
  if (document.hidden) {
    if (type === 'error') pendingError = message
    return
  }
  ElMessage[type]({ message, duration })
}

// flushPendingError 在页面恢复可见时弹出隐藏期间累积的错误消息。
// 由 focusGuard 的 visibilitychange 监听器调用。
export function flushPendingError() {
  if (!document.hidden && pendingError) {
    ElMessage.error({ message: pendingError, duration: MESSAGE_DURATION })
    pendingError = ''
  }
}

export const msg = {
  success: (message: string, duration = MESSAGE_DURATION) => showMessage('success', message, duration),
  error: (message: string, duration = MESSAGE_DURATION) => showMessage('error', message, duration),
  warning: (message: string, duration = MESSAGE_DURATION) => showMessage('warning', message, duration)
}

export const handleError = (error: unknown, defaultMsg = '操作失败') => {
  let message: string
  if (error instanceof AxiosError) {
    message = (error.response?.data as { error?: string })?.error || error.message || defaultMsg
  } else if (error instanceof Error) {
    message = error.message || defaultMsg
  } else {
    message = String(error) || defaultMsg
  }
  msg.error(message)
}
