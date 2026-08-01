<template>
  <div class="node-toolbar">
    <div class="toolbar-left">
      <input
        class="search-input"
        v-model="keyword"
        placeholder="搜索节点"
      />
      <div class="subscription-select">
        <BaseSelect
          :model-value="String(subscriptionFilter || '')"
          :options="subscriptionSelectOptions"
          placeholder="订阅"
          @update:model-value="v => subscriptionFilter = v ? Number(v) : ''"
        />
      </div>
      <div class="small-select">
        <BaseSelect
          :model-value="protocolFilter"
          :options="protocolOptions"
          placeholder="协议"
          @update:model-value="value => protocolFilter = value"
        />
      </div>
    </div>
    <div class="toolbar-right">
      <IconButton
        v-for="btn in speedButtons"
        :key="btn.event"
        :tooltip="btn.tooltip"
        :disabled="btn.disabled"
        :working="btn.working"
        :tone="btn.tone"
        @click="onButtonClick(btn.event)"
      >
        <component :is="btn.icon" />
      </IconButton>
      <span class="toolbar-divider" />
      <IconButton
        tooltip="更新订阅 (Alt+S)"
        :disabled="subscriptionUpdating || operationRunning"
        :working="subscriptionUpdating"
        tone="primary"
        @click="$emit('update-all-subscriptions')"
      >
        <Refresh />
      </IconButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Aim, CircleCheck, Delete, Refresh } from '@element-plus/icons-vue'
import { useNodeStore, useOperationStore, useSubscriptionStore } from '@/stores'
import { useNodeFilter } from '@/composables'
import { ALL_OPTION_LABEL, ALL_OPTION_VALUE } from '@/constants'
import IconButton from '@/components/IconButton.vue'
import BaseSelect from '@/components/BaseSelect.vue'

const props = defineProps<{
  subscriptionUpdating: boolean
  deleteLoading: boolean
}>()

const nodeStore = useNodeStore()
const subscriptionStore = useSubscriptionStore()
const operationStore = useOperationStore()
const subscriptions = computed(() => subscriptionStore.subscriptions)
const operationRunning = computed(() => operationStore.running)

const { keyword, protocolFilter, subscriptionFilter } = useNodeFilter({ refreshNodes: () => nodeStore.fetchNodes() })

const emit = defineEmits<{
  'update-all-subscriptions': []
  'speed-test-retest': []
  'speed-test-available': []
  'delete-timeout': []
}>()

const protocolOptions = computed(() => [
  { label: ALL_OPTION_LABEL, value: ALL_OPTION_VALUE },
  ...nodeStore.protocols
])

const subscriptionSelectOptions = computed(() => [
  { label: ALL_OPTION_LABEL, value: ALL_OPTION_VALUE },
  ...subscriptions.value.map(s => ({ label: s.name, value: String(s.id) }))
])

const activeSpeedAction = ref<string | null>(null)
watch(operationRunning, running => { if (!running) activeSpeedAction.value = null })

const batchDisabled = computed(() => operationRunning.value)

const speedButtons = computed(() => [
  { event: 'speed-test-retest' as const, icon: Aim, tooltip: '补测 (Alt+T)', disabled: batchDisabled.value, working: activeSpeedAction.value === 'speed-test-retest' && batchDisabled.value, tone: 'primary' as const },
  { event: 'speed-test-available' as const, icon: CircleCheck, tooltip: '重测可用 (Alt+V)', disabled: batchDisabled.value, working: activeSpeedAction.value === 'speed-test-available' && batchDisabled.value, tone: 'primary' as const },
  { event: 'delete-timeout' as const, icon: Delete, tooltip: '清理失效 (Alt+D)', disabled: props.deleteLoading || batchDisabled.value, working: props.deleteLoading, tone: 'danger' as const }
])

function onButtonClick(event: 'speed-test-retest' | 'speed-test-available' | 'delete-timeout') {
  activeSpeedAction.value = event
  switch (event) {
    case 'speed-test-retest':
      emit('speed-test-retest')
      break
    case 'speed-test-available':
      emit('speed-test-available')
      break
    case 'delete-timeout':
      emit('delete-timeout')
      break
  }
}
</script>

<style scoped>
.node-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-md);
  margin: var(--spacing-lg) var(--spacing-lg) var(--spacing-md);
  overflow: visible;
}

.toolbar-left,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-wrap: nowrap;
  min-width: 0;
}

.toolbar-left {
  flex: 1;
}

.toolbar-right {
  flex-shrink: 0;
}

.toolbar-divider {
  width: 1px;
  height: 20px;
  background: var(--border-soft);
  margin: 0 var(--spacing-xs);
  flex-shrink: 0;
}

.search-input {
  width: 200px;
  flex-shrink: 0;
  height: var(--row-height);
  padding: 0 var(--spacing-md);
  background: var(--surface);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-md);
  box-shadow: none;
  color: var(--text-secondary);
  font-size: var(--font-xs);
  font-weight: var(--weight-medium);
  outline: none;
  transition: var(--el-transition-duration);
}

.search-input::placeholder {
  color: var(--text-tertiary);
  font-weight: var(--weight-regular);
}

.search-input:hover,
.search-input:focus {
  border-color: var(--primary);
  background: var(--bg-hover);
  color: var(--text-primary);
}

.small-select {
  width: 122px;
  flex-shrink: 0;
}

.subscription-select {
  width: 152px;
  flex-shrink: 0;
}

@media (max-width: 1080px) {
  .node-toolbar {
    align-items: flex-start;
  }

  .toolbar-left {
    flex-wrap: wrap;
  }
}
</style>
