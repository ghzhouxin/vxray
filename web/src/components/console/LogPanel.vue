<template>
  <footer class="log-panel" :class="{ collapsed }">
    <div class="log-header" @click="handleHeaderClick">
      <div class="log-header-hit">
        <div class="log-header-title">日志</div>
      </div>
      <div v-if="!collapsed" class="log-filters" @click.stop>
        <input class="log-control log-keyword" :value="keyword" placeholder="搜索日志" @input="emit('update:keyword', ($event.target as HTMLInputElement).value)" />
        <BaseSelect
          class="log-filter-control"
          :model-value="level"
          :options="levelOptions"
          placeholder="等级"
          :display-label="levelLabel"
          menu-width="120px"
          height="28px"
          placement="top"
          @update:model-value="emit('update:level', $event)"
        />
        <BaseSelect
          class="log-filter-control tag-select"
          :model-value="tag"
          :options="tagOptions"
          placeholder="来源"
          :display-label="tagLabel"
          menu-width="160px"
          height="28px"
          placement="top"
          @update:model-value="emit('update:tag', $event)"
        />
        <IconButton
          :class="['log-refresh', { spinning: autoRefresh }]"
          :tooltip="autoRefresh ? '自动刷新：开启' : '自动刷新：暂停'"
          :tone="autoRefresh ? 'primary' : 'muted'"
          :size="ICON_BUTTON_SIZE_SM"
          @click="emit('update:autoRefresh', !autoRefresh)"
        >
          <RefreshRight />
        </IconButton>
        <IconButton class="log-clear" tooltip="清空日志" tone="danger" :size="ICON_BUTTON_SIZE_SM" @click="emit('clear')">
          <Delete />
        </IconButton>
      </div>
    </div>
    <div v-if="!collapsed" class="log-body" ref="logBodyRef">
      <div v-if="hasMore && loading" class="log-load-hint">加载历史中...</div>
      <div v-if="hasMore" ref="loadMoreSentinel" class="log-load-sentinel"></div>
      <div v-if="loading" class="log-state">加载中...</div>
      <div v-else-if="!logs.length" class="log-state">暂无日志</div>
      <template v-else>
        <div v-for="log in viewLogs" :key="log.id" class="log-line">
          <span class="log-time">{{ formatClock(log.updated_at || log.created_at) }}</span>
          <span class="log-level" :class="log.detailLevel || 'none'">{{ log.detailLevel || '-' }}</span>
          <span class="log-tag" :title="log.tag">{{ log.tag }}</span>
          <span class="log-message" :title="log.detail">{{ log.message }}</span>
        </div>
      </template>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Delete, RefreshRight } from '@element-plus/icons-vue'
import type { Log } from '@/types'
import { ICON_BUTTON_SIZE_LG, ICON_BUTTON_SIZE_SM } from '@/constants'
import { formatClock } from '@/utils/formatters'
import IconButton from '@/components/IconButton.vue'
import BaseSelect from '@/components/BaseSelect.vue'

const props = defineProps<{
  collapsed: boolean
  logs: Log[]
  tags: string[]
  levels: string[]
  loading: boolean
  hasMore: boolean
  level: string
  tag: string
  keyword: string
  autoRefresh: boolean
}>()

const emit = defineEmits<{
  'update:collapsed': [value: boolean]
  'update:level': [value: string]
  'update:tag': [value: string]
  'update:keyword': [value: string]
  'update:autoRefresh': [value: boolean]
  clear: []
  'load-more': []
}>()

const loadMoreSentinel = ref<HTMLElement>()
const logBodyRef = ref<HTMLElement>()
let observer: IntersectionObserver | null = null

function setupObserver() {
  if (observer) { observer.disconnect(); observer = null }
  if (!loadMoreSentinel.value || !logBodyRef.value) return
  observer = new IntersectionObserver(entries => {
    if (entries[0].isIntersecting && props.hasMore && !props.loading) {
      emit('load-more')
    }
  }, { root: logBodyRef.value, threshold: 0 })
  observer.observe(loadMoreSentinel.value)
}

watch([loadMoreSentinel, () => props.collapsed], () => nextTick(setupObserver))
onMounted(() => nextTick(setupObserver))
onBeforeUnmount(() => observer?.disconnect())

const levelOptions = computed(() => [
  { label: '全部', value: '' },
  ...props.levels.map(l => ({ label: l, value: l }))
])
const levelCache = new WeakMap<Log, string>()
const viewLogs = computed(() =>
  props.logs.map(log => ({
    ...log,
    detailLevel: resolveDetailLevel(log)
  }))
)
const tagOptions = computed(() => [
  { label: '全部', value: '' },
  ...props.tags.map(t => ({ label: t, value: t }))
])
const levelLabel = computed(() => props.level ? levelOptions.value.find(item => item.value === props.level)?.label || '等级' : '等级')
const tagLabel = computed(() => props.tag ? tagOptions.value.find(item => item.value === props.tag)?.label || '来源' : '来源')

function resolveDetailLevel(log: Log) {
  if (!log.detail) return ''
  if (levelCache.has(log)) return levelCache.get(log)!
  try {
    const parsed = JSON.parse(log.detail) as { level?: string }
    const result = parsed.level || ''
    levelCache.set(log, result)
    return result
  } catch {
    return ''
  }
}

function handleHeaderClick() {
  emit('update:collapsed', !props.collapsed)
}
</script>

<style scoped>
.log-panel {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 30;
  height: 300px;
  overflow: visible;
  background: var(--bg-input);
  border-top: 1px solid var(--border-soft);
  transition: height .2s;
}

.log-panel.collapsed {
  height: 0;
  border-top: none;
  overflow: hidden;
}

.log-header {
  height: var(--row-height);
  padding: 0 var(--spacing-lg);
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  background: rgba(18, 26, 35, .86);
  cursor: pointer;
  min-width: 0;
}

.log-header-hit {
  width: 72px;
  height: 100%;
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

.log-header-title {
  flex-shrink: 0;
  color: var(--text-secondary);
  font-weight: var(--weight-bold);
  user-select: none;
}

.log-control {
  height: var(--row-height-sm);
  padding: 0 var(--spacing-sm);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-md);
  background: var(--bg-base);
  color: var(--text-secondary);
  font-size: var(--font-xs);
  box-sizing: border-box;
  transition: var(--el-transition-duration);
}

.log-control:hover {
  color: var(--text-primary);
  border-color: var(--primary);
  background: var(--bg-hover);
}

.log-filters {
  margin-left: auto;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--spacing-sm);
  min-width: 0;
  flex: 0 0 auto;
}

.log-filter-control {
  width: 96px;
  flex-shrink: 0;
}

.log-filter-control.tag-select {
  width: 140px;
}

.log-filter-control :deep(.select-trigger) {
  width: 100%;
  padding: 0 var(--spacing-sm);
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  border-radius: var(--radius-md);
  background: var(--bg-base);
  font-size: var(--font-xs);
  line-height: 1;
  box-sizing: border-box;
}

.log-filter-control :deep(.select-menu) {
  z-index: 35;
}

.log-keyword {
  width: 168px;
  min-width: 0;
  outline: none;
}

.log-clear {
  white-space: nowrap;
}

.log-refresh.spinning :deep(svg) {
  animation: log-spin 2s linear infinite;
}

@keyframes log-spin {
  to { transform: rotate(360deg); }
}

.log-body {
  height: calc(100% - var(--row-height));
  overflow-y: auto;
  padding: var(--spacing-sm) var(--spacing-lg) var(--spacing-md);
  font-family: var(--mono);
  font-size: var(--font-xs);
}

.log-load-sentinel {
  height: 1px;
}

.log-load-hint {
  padding: var(--spacing-xs) 0;
  color: var(--text-tertiary);
  font-size: var(--font-xs);
  text-align: center;
}

.log-line {
  display: grid;
  grid-template-columns: 54px 38px 48px 1fr;
  gap: var(--spacing-sm);
  padding: var(--spacing-xs) 0;
  align-items: start;
  border-bottom: 1px dashed var(--border-soft);
}

.log-state {
  height: 100%;
  min-height: 84px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-tertiary);
}

.log-message {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-time,
.log-tag {
  color: var(--text-tertiary);
}

.log-tag,
.log-level,
.log-time {
  white-space: nowrap;
}

.log-level {
  text-transform: uppercase;
}

.log-level.info {
  color: var(--primary);
}

.log-level.warn {
  color: var(--warning);
}

.log-level.error {
  color: var(--danger);
}

.log-level.debug,
.log-level.none {
  color: var(--text-tertiary);
}
</style>
