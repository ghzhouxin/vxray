import { type Ref } from 'vue'
import { PROXY_READY_DELAY } from '@/constants'

export async function withLoading<T>(loadingRef: Ref<boolean>, fn: () => Promise<T>): Promise<T> {
  loadingRef.value = true
  try { return await fn() }
  finally { loadingRef.value = false }
}

export function waitForProxyReady(): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, PROXY_READY_DELAY))
}
