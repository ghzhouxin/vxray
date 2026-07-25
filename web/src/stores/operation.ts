import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type { OperationProgress, OperationType } from '@/types'

export const useOperationStore = defineStore('operation', () => {
  const active = ref<OperationProgress | null>(null)

  const running = computed(() => active.value?.status === 'running')
  const type = computed(() => active.value?.type || '')

  function start(operationType: OperationType, message = '') {
    active.value = {
      type: operationType,
      status: 'running',
      total: 0,
      completed: 0,
      success: 0,
      failed: 0,
      message
    }
  }

  function applyProgress(progress: OperationProgress) {
    active.value = progress
  }

  function clear() {
    active.value = null
  }

  return {
    active, running, type,
    start, applyProgress, clear
  }
})
