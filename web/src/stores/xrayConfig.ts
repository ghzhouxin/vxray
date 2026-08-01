import { defineStore } from 'pinia'
import { ref } from 'vue'
import { xrayApi } from '@/api'
import { withLoading } from '@/utils/async'

export const useXrayConfigStore = defineStore('xrayConfig', () => {
  const xrayConfigText = ref('')
  const originalXrayConfigText = ref('')
  const defaultXrayConfigText = ref('')
  const loading = ref(false)

  async function fetchXrayConfig() {
    const [content, defaultContent] = await Promise.all([xrayApi.getConfig(), xrayApi.getDefaultConfig()])
    xrayConfigText.value = content.content
    originalXrayConfigText.value = xrayConfigText.value
    defaultXrayConfigText.value = defaultContent.content
  }

  async function saveXrayConfig() {
    await withLoading(loading, async () => {
      await xrayApi.saveConfig(xrayConfigText.value)
      originalXrayConfigText.value = xrayConfigText.value
    })
  }

  function resetXrayConfig() { xrayConfigText.value = originalXrayConfigText.value }
  function restoreDefaultXrayConfig() { xrayConfigText.value = defaultXrayConfigText.value }

  return {
    xrayConfigText, originalXrayConfigText,
    loading,
    fetchXrayConfig, saveXrayConfig, resetXrayConfig, restoreDefaultXrayConfig
  }
})
