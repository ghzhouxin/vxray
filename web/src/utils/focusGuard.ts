import { ElMessage } from 'element-plus'
import { MESSAGE_DURATION } from '@/constants'

// 模块加载时一次性捕获 native focus 作为唯一真相源，避免 blur/focus 快速切换时捕获到抑制函数
const nativeFocus = HTMLElement.prototype.focus
let suppressed = false
let pendingError = ''

export function initFocusGuard() {
  if (typeof window === 'undefined') return
  // window blur/focus 在元素 focusout/focusin 之前触发，确保 patch 在 Element Plus focus trap 反应前生效
  window.addEventListener('blur', handleWindowBlur, { capture: true })
  window.addEventListener('focus', handleWindowFocus, { capture: true })
  document.addEventListener('visibilitychange', handleVisibilityChange)
}

function handleWindowBlur() {
  // 窗口失焦时立即 patch，阻止 focus trap 后续的 tryFocus → el.focus() 激活窗口
  if (!suppressed) {
    HTMLElement.prototype.focus = function () { /* suppressed: window blurred */ }
    suppressed = true
  }
}

function handleWindowFocus() {
  // 窗口恢复焦点时清除标志，延迟恢复 native focus 避免与 focus trap 的 setTimeout(0) 竞争
  if (suppressed) {
    suppressed = false
    setTimeout(() => {
      // 若期间又 blur 了（快速切换），则不恢复
      if (!suppressed) HTMLElement.prototype.focus = nativeFocus
    }, 0)
  }
}

function handleVisibilityChange() {
  // 页面恢复可见时刷新隐藏期间累积的错误消息
  if (!document.hidden && pendingError) {
    ElMessage.error({ message: pendingError, duration: MESSAGE_DURATION })
    pendingError = ''
  }
}

/** 供 msg 模块使用的错误缓存接口 */
export function setPendingError(message: string) { pendingError = message }
