<template>
  <AppDialog :model-value="modelValue" title="测速网站管理" class="speedtest-targets-dialog" @update:model-value="emit('update:modelValue', $event)">
    <div class="manager">
      <div class="toolbar">
        <IconButton label="添加网站" :size="ICON_BUTTON_SIZE_LG" tone="primary" :disabled="configStore.settingsSaving" @click="startCreate"><Plus /></IconButton>
      </div>

      <div class="list">
        <div v-if="expandedKey === 'new'" class="card expanded">
          <div class="card-head">
            <strong class="card-title">新增网站</strong>
            <div class="card-actions">
              <IconButton label="取消" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="cancelEdit"><Close /></IconButton>
              <IconButton label="保存" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="configStore.settingsSaving" :disabled="configStore.settingsSaving" @click="submit"><Check /></IconButton>
            </div>
          </div>
          <div class="card-body">
            <el-form :model="form" label-position="top">
              <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
              <el-form-item label="URL"><el-input v-model="form.url" /></el-form-item>
            </el-form>
          </div>
        </div>

        <div v-for="(target, idx) in targets" :key="target.url" class="card" :class="{ expanded: expandedKey === idx }">
          <div class="card-head">
            <strong class="card-title">{{ target.name }}</strong>
            <div class="card-actions">
              <template v-if="expandedKey === idx">
                <IconButton label="取消" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="cancelEdit"><Close /></IconButton>
                <IconButton label="保存" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="configStore.settingsSaving" :disabled="configStore.settingsSaving" @click="submit"><Check /></IconButton>
              </template>
              <template v-else>
                <IconButton tooltip="编辑" :size="ICON_BUTTON_SIZE_SM" tone="muted" :disabled="configStore.settingsSaving" @click="handleStartEdit(idx)"><EditPen /></IconButton>
                <IconButton tooltip="删除" :size="ICON_BUTTON_SIZE_SM" tone="danger" :disabled="configStore.settingsSaving" @click="handleDelete(idx)"><Delete /></IconButton>
              </template>
            </div>
          </div>
          <div v-if="expandedKey !== idx" class="card-meta">{{ target.url }}</div>
          <div v-if="expandedKey === idx" class="card-body">
            <el-form :model="form" label-position="top">
              <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
              <el-form-item label="URL"><el-input v-model="form.url" /></el-form-item>
            </el-form>
          </div>
        </div>

        <div v-if="!targets.length && expandedKey !== 'new'" class="empty">
          暂无测速网站，点击上方"添加网站"创建
        </div>
      </div>
    </div>
  </AppDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Check, Close, Delete, EditPen, Plus } from '@element-plus/icons-vue'
import type { UserSettings } from '@/types'
import { useSettingsStore } from '@/stores'
import { ICON_BUTTON_SIZE_LG, ICON_BUTTON_SIZE_SM } from '@/constants'
import { msg } from '@/utils/message'
import { useCardEditor } from '@/composables/useCardEditor'
import AppDialog from '@/components/AppDialog.vue'
import IconButton from '@/components/IconButton.vue'

type Target = UserSettings['speedtest']['website_targets'][number]

defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: []
}>()

const configStore = useSettingsStore()
const targets = computed(() => configStore.settings.speedtest.website_targets)

const { expandedKey, form, resetForm, startCreate, startEdit, cancelEdit } = useCardEditor<Target>({ name: '', url: '' })

function handleStartEdit(idx: number) {
  const target = targets.value[idx]
  startEdit(idx, { name: target.name, url: target.url })
}

async function submit() {
  const name = form.name.trim()
  const url = form.url.trim()
  if (!name) { msg.warning('请输入名称'); return }
  if (!url) { msg.warning('请输入 URL'); return }

  const next = [...targets.value]
  if (expandedKey.value === 'new') {
    next.push({ name, url })
  } else if (typeof expandedKey.value === 'number') {
    next[expandedKey.value] = { name, url }
  }
  configStore.settings.speedtest.website_targets = next
  expandedKey.value = null
  resetForm()
  emit('save')
}

function handleDelete(idx: number) {
  configStore.settings.speedtest.website_targets = targets.value.filter((_, i) => i !== idx)
  emit('save')
}
</script>

<style scoped>
.manager .card-meta {
  word-break: break-all;
}
</style>

<style>
.app-dialog.speedtest-targets-dialog .el-dialog__body { padding-top: var(--spacing-sm) !important; }
</style>
