<template>
  <AppDialog :model-value="modelValue" title="系统设置" @update:model-value="emit('update:modelValue', $event)">
    <div v-loading="configStore.loading" class="settings-panel">
      <label>测速地址<el-input v-model="configStore.settings.speedtest.target_url" /></label>
      <label>超时时间<el-input-number v-model="configStore.settings.speedtest.timeout" :min="SPEEDTEST_TIMEOUT_MIN" :max="SPEEDTEST_TIMEOUT_MAX" :step="SPEEDTEST_TIMEOUT_STEP" /></label>
      <label>并发数量<el-input-number v-model="configStore.settings.speedtest.concurrency" :min="1" :max="SPEEDTEST_CONCURRENCY_MAX" :step="SPEEDTEST_CONCURRENCY_STEP" /></label>

      <label>Geo 来源<BaseSelect :model-value="configStore.settings.geo.selected_source" :options="geoSourceOptions" placeholder="Geo 来源" @update:model-value="v => configStore.settings.geo.selected_source = v" /></label>
      <GeoFileManager />
    </div>
    <template #footer>
      <div class="settings-footer">
        <IconButton block label="重置" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="handleResetDefault"><RefreshLeft /></IconButton>
        <IconButton block label="取消" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="emit('update:modelValue', false)"><Close /></IconButton>
        <IconButton block label="保存" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="configStore.settingsSaving" :disabled="configStore.settingsSaving" @click="emit('save')"><Check /></IconButton>
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

const configStore = useSettingsStore()
const geoSourceNames = computed(() => Object.keys(configStore.systemMeta.assets.geo_sources || {}))
const geoSourceOptions = computed(() => geoSourceNames.value.map(name => ({ label: name, value: name })))

function handleResetDefault() {
  configStore.restoreDefaultUserSettings()
  msg.success('已恢复默认值，请保存以生效')
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
