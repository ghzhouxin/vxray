import { type Ref } from 'vue'

export async function withLoading<T>(loadingRef: Ref<boolean>, fn: () => Promise<T>): Promise<T> {
  loadingRef.value = true
  try { return await fn() }
  finally { loadingRef.value = false }
}
