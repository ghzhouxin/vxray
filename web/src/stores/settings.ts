import { defineStore } from 'pinia'
import { ref } from 'vue'
import { settingsApi } from '@/api'
import { withLoading } from '@/utils/async'
import type { SystemMeta, UserSettings } from '@/types'

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

  function restoreDefaultUserSettings() { settings.value = structuredClone(defaultSettings.value) }

  return {
    settings, defaultSettings, systemMeta,
    loading, settingsSaving,
    fetchConfigView, saveUserSettings, restoreDefaultUserSettings
  }
})
