import { ref, onMounted, onUnmounted } from 'vue'

export function useAutoRefresh(
  fn: () => Promise<void> | void,
  interval: number,
  options: { autoStart?: boolean; pauseOnHidden?: boolean; onError?: (e: unknown) => void } = {}
) {
  const { autoStart = true, pauseOnHidden = true, onError } = options
  const timer = ref<ReturnType<typeof setInterval> | null>(null)
  const isRunning = ref(false)
  let wasRunning = false

  function run() {
    const result = fn()
    if (result instanceof Promise) result.catch(e => { onError?.(e) })
  }

  function start() {
    if (timer.value) return
    timer.value = setInterval(run, interval)
    isRunning.value = true
  }

  function stop() {
    if (timer.value) { clearInterval(timer.value); timer.value = null }
    isRunning.value = false
  }

  function handleVisibility() {
    if (!pauseOnHidden) return
    if (document.hidden) { wasRunning = isRunning.value; stop(); return }
    if (wasRunning) {
      start()
      run()
    }
  }

  if (autoStart) onMounted(start)
  if (pauseOnHidden) {
    onMounted(() => document.addEventListener('visibilitychange', handleVisibility))
    onUnmounted(() => document.removeEventListener('visibilitychange', handleVisibility))
  }
  onUnmounted(stop)

  return { start, stop, isRunning }
}
