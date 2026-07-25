<template>
  <AppDialog :model-value="modelValue" title="测速网站管理" class="speedtest-targets-dialog" @update:model-value="emit('update:modelValue', $event)">
    <div class="manager">
      <div class="toolbar">
        <IconButton label="添加网站" :size="ICON_BUTTON_SIZE_LG" tone="primary" :disabled="settingsStore.settingsSaving" @click="startCreate"><Plus /></IconButton>
      </div>

      <div class="list">
        <div v-if="expandedKey === 'new'" class="card expanded">
          <div class="card-head">
            <strong class="card-title">新增网站</strong>
            <div class="card-actions">
              <IconButton label="取消" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="cancelEdit"><Close /></IconButton>
              <IconButton label="保存" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="settingsStore.settingsSaving" :disabled="settingsStore.settingsSaving" @click="handleSubmit"><Check /></IconButton>
            </div>
          </div>
          <div class="card-body">
            <el-form :model="form" label-position="top">
              <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
              <el-form-item label="URL"><el-input v-model="form.url" /></el-form-item>
              <el-form-item label="图标 URL"><el-input v-model="form.icon" placeholder="留空则显示名称" /></el-form-item>
            </el-form>
          </div>
        </div>

        <div v-for="(target, idx) in targets" :key="target.url" class="card" :class="{ expanded: expandedKey === idx }">
          <div class="card-head">
            <AppIcon :src="target.icon" :name="target.name" :size="18" />
            <strong class="card-title">{{ target.name }}</strong>
            <div class="card-actions">
              <template v-if="expandedKey === idx">
                <IconButton label="取消" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="cancelEdit"><Close /></IconButton>
                <IconButton label="保存" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="settingsStore.settingsSaving" :disabled="settingsStore.settingsSaving" @click="handleSubmit"><Check /></IconButton>
              </template>
              <template v-else>
                <IconButton tooltip="编辑" :size="ICON_BUTTON_SIZE_SM" tone="muted" :disabled="settingsStore.settingsSaving" @click="handleStartEdit(idx)"><EditPen /></IconButton>
                <IconButton tooltip="删除" :size="ICON_BUTTON_SIZE_SM" tone="danger" :disabled="settingsStore.settingsSaving" @click="handleDelete(idx)"><Delete /></IconButton>
              </template>
            </div>
          </div>
          <div v-if="expandedKey !== idx" class="card-meta">{{ target.url }}</div>
          <div v-if="expandedKey === idx" class="card-body">
            <el-form :model="form" label-position="top">
              <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
              <el-form-item label="URL"><el-input v-model="form.url" /></el-form-item>
              <el-form-item label="图标 URL"><el-input v-model="form.icon" placeholder="留空则显示名称" /></el-form-item>
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
import { useSettingsStore } from '@/stores'
import { ICON_BUTTON_SIZE_LG, ICON_BUTTON_SIZE_SM } from '@/constants'
import { msg } from '@/utils/message'
import { useCardEditor } from '@/composables/useCardEditor'
import AppDialog from '@/components/AppDialog.vue'
import AppIcon from '@/components/AppIcon.vue'
import IconButton from '@/components/IconButton.vue'

type TargetForm = { name: string; url: string; icon: string }

defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const settingsStore = useSettingsStore()
const targets = computed(() => settingsStore.settings.speedtest.website_targets)

const { expandedKey, form, resetForm, startCreate, startEdit, cancelEdit } = useCardEditor<TargetForm>({ name: '', url: '', icon: '' })

function handleStartEdit(idx: number) {
  const target = targets.value[idx]
  startEdit(idx, { name: target.name, url: target.url, icon: target.icon || '' })
}

async function handleSubmit() {
  const name = form.name.trim()
  const url = form.url.trim()
  const icon = (form.icon || '').trim()
  if (!name) { msg.warning('请输入名称'); return }
  if (!url) { msg.warning('请输入 URL'); return }

  const previous = [...targets.value]
  const next = [...targets.value]
  if (expandedKey.value === 'new') {
    next.push({ name, url, latency: 0, ...(icon ? { icon } : {}) })
  } else if (typeof expandedKey.value === 'number') {
    const existing = targets.value[expandedKey.value]
    next[expandedKey.value] = { ...existing, name, url, icon: icon || undefined }
  }
  settingsStore.updateWebsiteTargets(next)
  try {
    await settingsStore.saveUserSettings()
    expandedKey.value = null
    resetForm()
  } catch (e) {
    settingsStore.settings.speedtest.website_targets = previous
    console.warn('[SpeedTestTargetsDialog] 保存失败', e)
    msg.error('保存失败')
  }
}

async function handleDelete(idx: number) {
  const previous = [...targets.value]
  settingsStore.updateWebsiteTargets(targets.value.filter((_, i) => i !== idx))
  try {
    await settingsStore.saveUserSettings()
  } catch (e) {
    settingsStore.settings.speedtest.website_targets = previous
    console.warn('[SpeedTestTargetsDialog] 删除失败', e)
    msg.error('删除失败')
  }
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
