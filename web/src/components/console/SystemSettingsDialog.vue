<template>
  <AppDialog :model-value="modelValue" title="系统设置" @update:model-value="emit('update:modelValue', $event)">
    <div v-loading="settingsStore.loading" class="settings-panel">
      <label>测速地址<el-input v-model="settingsStore.settings.speedtest.target_url" /></label>
      <label>超时时间<el-input-number v-model="settingsStore.settings.speedtest.timeout" :min="SPEEDTEST_TIMEOUT_MIN" :max="SPEEDTEST_TIMEOUT_MAX" :step="SPEEDTEST_TIMEOUT_STEP" /></label>
      <label>并发数量<el-input-number v-model="settingsStore.settings.speedtest.concurrency" :min="1" :max="SPEEDTEST_CONCURRENCY_MAX" :step="SPEEDTEST_CONCURRENCY_STEP" /></label>

      <label>Geo 来源<BaseSelect :model-value="settingsStore.settings.geo.selected_source" :options="geoSourceOptions" placeholder="Geo 来源" @update:model-value="v => settingsStore.settings.geo.selected_source = v" /></label>
      <GeoFileManager />
    </div>
    <template #footer>
      <div class="settings-footer">
        <IconButton block label="重置" :size="ICON_BUTTON_SIZE_SM" tone="muted" :working="settingsStore.settingsSaving" :disabled="settingsStore.settingsSaving" @click="handleResetDefault"><RefreshLeft /></IconButton>
        <IconButton block label="取消" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="emit('update:modelValue', false)"><Close /></IconButton>
        <IconButton block label="保存" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="settingsStore.settingsSaving" :disabled="settingsStore.settingsSaving" @click="emit('save')"><Check /></IconButton>
      </div>
    </template>
  </AppDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Check, Close, RefreshLeft } from '@element-plus/icons-vue'
import GeoFileManager from './GeoFileManager.vue'
import { useSettingsStore } from '@/stores'
import {
  SPEEDTEST_TIMEOUT_MIN, SPEEDTEST_TIMEOUT_MAX, SPEEDTEST_TIMEOUT_STEP,
  SPEEDTEST_CONCURRENCY_MAX, SPEEDTEST_CONCURRENCY_STEP, ICON_BUTTON_SIZE_SM
} from '@/constants'
import { msg } from '@/utils/message'
import AppDialog from '@/components/AppDialog.vue'
import IconButton from '@/components/IconButton.vue'
import BaseSelect from '@/components/BaseSelect.vue'

defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: []
}>()

const settingsStore = useSettingsStore()
const geoSourceNames = computed(() => Object.keys(settingsStore.systemMeta.assets.geo_sources || {}))
const geoSourceOptions = computed(() => geoSourceNames.value.map(name => ({ label: name, value: name })))

async function handleResetDefault() {
  try {
    await settingsStore.resetAndSaveUserSettings()
    msg.success('已重置为默认值并保存')
  } catch (e) {
    console.warn('[SystemSettingsDialog] 重置失败', e)
    msg.error('重置失败')
  }
}
</script>

<style scoped>
.settings-panel {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.settings-panel label {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm) var(--spacing-xs);
  color: var(--text-secondary);
}

.settings-footer { display: flex; gap: var(--spacing-sm); }
.settings-footer :deep(.icon-button) { flex: 1; }
</style>
