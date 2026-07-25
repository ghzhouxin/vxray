import { onBeforeUnmount } from 'vue'

const closeHandlers = new Set<() => void>()

export function useSelectCoordinator(close: () => void) {
  function closeOthers() {
    closeHandlers.forEach(h => { if (h !== close) h() })
  }

  onBeforeUnmount(() => closeHandlers.delete(close))

  return { closeOthers, register: () => closeHandlers.add(close) }
}
