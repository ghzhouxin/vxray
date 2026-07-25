import { onBeforeUnmount, onMounted, type Ref } from 'vue'

export function useClickOutside(refs: Ref<HTMLElement | undefined> | Ref<HTMLElement | undefined>[], callback: () => void) {
  const refList = Array.isArray(refs) ? refs : [refs]
  function handler(event: MouseEvent) {
    const target = event.target as HTMLElement
    if (refList.every(r => !r.value?.contains(target))) callback()
  }
  onMounted(() => window.addEventListener('click', handler))
  onBeforeUnmount(() => window.removeEventListener('click', handler))
}
