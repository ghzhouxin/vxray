import { defineStore } from 'pinia'
import { ref } from 'vue'
import { settingsApi } from '@/api'
import { withLoading } from '@/utils/async'
import type { SpeedTestTarget, SystemMeta, UserSettings } from '@/types'

function createUserSettings(): UserSettings {
  return {
    speedtest: { target_url: '', timeout: 0, concurrency: 0, website_targets: [] },
    geo: { selected_source: '' }
  }
}

function createSystemMeta(): SystemMeta {
  return {
    home: '',
    paths: {
      geo_dir: '',
      xray_config_path: ''
    },
    server: { host: '', port: 0 },
    xray: { binary: '' },
    assets: { geo_sources: {} }
  }
}

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<UserSettings>(createUserSettings())
  const defaultSettings = ref<UserSettings>(createUserSettings())
  const systemMeta = ref<SystemMeta>(createSystemMeta())
  const loading = ref(false)
  const settingsSaving = ref(false)

  async function fetchConfigView() {
    await withLoading(loading, async () => {
      const [view, defaults] = await Promise.all([settingsApi.get(), settingsApi.getDefault()])
      Object.assign(settings.value, view.settings)
      Object.assign(defaultSettings.value, defaults)
      Object.assign(systemMeta.value, view.system)
    })
  }

  async function saveUserSettings() {
    await withLoading(settingsSaving, async () => {
      await settingsApi.update(settings.value)
    })
  }

  // resetAndSaveUserSettings 从后端拿最新默认值，直接保存到后端并刷新前端 state。
  // 一步到位，不依赖前端缓存的 defaultSettings，不需要用户手动点保存。
  async function resetAndSaveUserSettings() {
    await withLoading(settingsSaving, async () => {
      const defaults = await settingsApi.getDefault()
      await settingsApi.update(defaults)
      Object.assign(settings.value, defaults)
      Object.assign(defaultSettings.value, defaults)
    })
  }

  function updateWebsiteTargets(targets: SpeedTestTarget[]) {
    settings.value.speedtest.website_targets = targets
  }

  return {
    settings, systemMeta,
    loading, settingsSaving,
    fetchConfigView, saveUserSettings, resetAndSaveUserSettings, updateWebsiteTargets
  }
})
