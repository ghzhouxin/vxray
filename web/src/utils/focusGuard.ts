import { flushPendingError } from './message'

let suppressed = false

export function initFocusGuard() {
  if (typeof window === 'undefined') return
  // window blur/focus 在元素 focusout/focusin 之前触发，确保抑制在 Element Plus focus trap 反应前生效
  window.addEventListener('blur', handleWindowBlur, { capture: true })
  window.addEventListener('focus', handleWindowFocus, { capture: true })
  document.addEventListener('visibilitychange', handleVisibilityChange)
}

function handleWindowBlur() {
  // 窗口失焦时监听 focusin，阻止 focus trap 后续的 tryFocus → el.focus() 激活窗口
  if (!suppressed) {
    suppressed = true
    document.addEventListener('focusin', suppressFocusIn, { capture: true })
  }
}

function handleWindowFocus() {
  if (suppressed) {
    suppressed = false
    document.removeEventListener('focusin', suppressFocusIn, { capture: true })
  }
}

// 窗口失焦期间，若元素获得焦点则立即 blur 并阻断 focusout 传播，
// 防止 Element Plus focus trap 拉回焦点导致循环。
function suppressFocusIn(e: FocusEvent) {
  const target = e.target as HTMLElement | null
  if (!target || target === document.body) return
  document.addEventListener('focusout', blockFocusOut, { capture: true, once: true })
  target.blur()
}

function blockFocusOut(e: FocusEvent) {
  e.stopImmediatePropagation()
}

function handleVisibilityChange() {
  flushPendingError()
}
