<template>
  <AppDialog :model-value="modelValue" title="订阅管理" class="subscription-dialog" @update:model-value="emit('update:modelValue', $event)">
    <div class="manager">
      <div class="toolbar">
        <IconButton label="添加订阅" :size="ICON_BUTTON_SIZE_LG" tone="primary" @click="startCreate"><Plus /></IconButton>
        <IconButton label="更新全部" :size="ICON_BUTTON_SIZE_LG" tone="primary" :working="updatingAll" :disabled="updatingAll" @click="emit('update-all')"><Refresh /></IconButton>
      </div>

      <div class="list">
        <div v-if="expandedKey === 'new'" class="card expanded">
          <div class="card-head">
            <strong class="card-title">新增订阅</strong>
            <div class="card-actions">
              <IconButton label="取消" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="cancelEdit"><Close /></IconButton>
              <IconButton label="保存" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="submitting" :disabled="submitting" @click="submit"><Check /></IconButton>
            </div>
          </div>
          <div class="card-body">
            <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
              <el-form-item label="名称" prop="name"><el-input v-model="form.name" /></el-form-item>
              <el-form-item label="订阅地址" prop="url"><el-input v-model="form.url" type="textarea" :rows="3" /></el-form-item>
            </el-form>
          </div>
        </div>

        <div v-for="sub in subscriptionStore.subscriptions" :key="sub.id" class="card" :class="{ expanded: expandedKey === sub.id }">
          <div class="card-head">
            <strong class="card-title">{{ sub.name }}<span v-if="updatingId === sub.id" class="status-running">更新中</span></strong>
            <div class="card-actions">
              <template v-if="expandedKey === sub.id">
                <IconButton label="取消" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="cancelEdit"><Close /></IconButton>
                <IconButton label="保存" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="submitting" :disabled="submitting" @click="submit"><Check /></IconButton>
              </template>
              <template v-else>
                <IconButton tooltip="更新" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="updatingId === sub.id" :disabled="updatingId === sub.id" @click="emit('update-subscription', sub.id)"><Refresh /></IconButton>
                <IconButton tooltip="编辑" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="handleStartEdit(sub)"><EditPen /></IconButton>
                <IconButton tooltip="删除" :size="ICON_BUTTON_SIZE_SM" tone="danger" @click="handleDelete(sub)"><Delete /></IconButton>
              </template>
            </div>
          </div>
          <div v-if="expandedKey !== sub.id" class="card-meta">
            <span>{{ sub.node_count }} 节点</span>
            <span>{{ formatSyncTime(sub.last_sync_at) }}</span>
            <span v-if="sub.last_sync_status === 'failed'" class="status-failed">更新失败</span>
          </div>
          <div v-if="expandedKey === sub.id" class="card-body">
            <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
              <el-form-item label="名称" prop="name"><el-input v-model="form.name" /></el-form-item>
              <el-form-item label="订阅地址" prop="url"><el-input v-model="form.url" type="textarea" :rows="3" /></el-form-item>
            </el-form>
          </div>
        </div>
        <div v-if="!subscriptionStore.subscriptions.length && expandedKey !== 'new'" class="empty">
          暂无订阅，点击上方"添加订阅"创建
        </div>
      </div>
    </div>
  </AppDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, Close, Delete, EditPen, Plus, Refresh } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import type { Subscription, SubscriptionFormData } from '@/types'
import { useSubscriptionStore } from '@/stores'
import { ICON_BUTTON_SIZE_LG, ICON_BUTTON_SIZE_SM } from '@/constants'
import { formatTime } from '@/utils/formatters'
import { useCardEditor } from '@/composables/useCardEditor'
import AppDialog from '@/components/AppDialog.vue'
import IconButton from '@/components/IconButton.vue'

const props = defineProps<{
  modelValue: boolean
  updatingId: number
  updatingAll: boolean
  submitting: boolean
}>()

const subscriptionStore = useSubscriptionStore()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  submit: [id: number | null, data: SubscriptionFormData]
  'update-subscription': [id: number]
  'update-all': []
  'delete-subscription': [subscription: Subscription]
}>()

const formRef = ref<FormInstance>()
const { expandedKey, form, resetForm, startCreate, startEdit, cancelEdit } = useCardEditor<SubscriptionFormData>({ name: '', url: '' })
const editingId = computed(() => typeof expandedKey.value === 'number' ? expandedKey.value : null)
const rules: FormRules = {
  name: [{ required: true, message: '请输入订阅名称', trigger: 'blur' }],
  url: [{ required: true, message: '请输入订阅地址', trigger: 'blur' }]
}

function handleStartEdit(sub: Subscription) {
  startEdit(sub.id, { name: sub.name, url: sub.url })
}

async function submit() {
  if (!formRef.value) return
  try { await formRef.value.validate() } catch { return }
  emit('submit', editingId.value, { name: form.name, url: form.url })
}

function handleDelete(sub: Subscription) {
  emit('delete-subscription', sub)
}

function formatSyncTime(value?: string) {
  return value ? formatTime(value) : '未同步'
}

watch(() => props.submitting, (value, prev) => {
  if (prev && !value) {
    expandedKey.value = null
    resetForm()
  }
})
</script>

<style scoped>
.status-running,
.status-failed {
  font-size: var(--font-xs);
  font-weight: var(--weight-medium);
}

.status-running { color: var(--warning); }
.status-failed { color: var(--danger); }
</style>

<style>
.app-dialog.subscription-dialog .el-dialog__body { padding-top: var(--spacing-sm) !important; }
</style>
