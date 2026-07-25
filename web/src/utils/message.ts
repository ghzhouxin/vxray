import { AxiosError } from 'axios'
import { MESSAGE_DURATION } from '@/constants'
import { ElMessage } from 'element-plus'
import { setPendingError } from './focusGuard'

function showMessage(type: 'success' | 'error' | 'warning', message: string, duration: number) {
  if (document.hidden) {
    if (type === 'error') setPendingError(message)
    return
  }
  ElMessage[type]({ message, duration })
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
