<template>
  <div class="node-table-shell" @scroll="handleScroll">
    <table class="node-table">
      <colgroup>
        <col class="col-name" />
        <col class="col-address" />
        <col class="col-protocol" />
        <col class="col-transport" />
        <col class="col-latency" />
        <col class="col-actions" />
      </colgroup>
      <thead>
        <tr>
          <th>节点</th>
          <th>地址</th>
          <th>协议</th>
          <th>传输</th>
          <th>延迟</th>
          <th class="actions-col">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="topPadding > 0" class="spacer-row"><td :colspan="6" :style="{ height: `${topPadding}px` }"></td></tr>
        <tr
          v-for="row in visibleRows"
          :key="row.id"
          :class="{ active: isCurrent(row) }"
          data-log-keep-open
          @click="emit('use-node', row)"
        >
          <td :title="row.name">
            <div class="node-name">
              <span class="node-name-text">{{ row.name }}</span>
            </div>
          </td>
          <td class="node-endpoint" :title="`${row.address}:${row.port}`">{{ row.address }}:{{ row.port }}</td>
          <td class="node-protocol">{{ row.protocol_label }}</td>
          <td class="node-transport" :title="row.transportLabel">{{ row.transportLabel }}</td>
          <td class="node-latency" :class="getLatencyClass(row.latency)">{{ formatLatency(row.latency) }}</td>
          <td class="actions-col" @click.stop>
            <IconButton class="row-action" tooltip="测速" :disabled="speedTestRunning" tone="primary" :size="ICON_BUTTON_SIZE_SM" @click="emit('speed-test-node', row)">
              <Aim />
            </IconButton>
            <IconButton class="row-action" tooltip="删除" tone="danger" :size="ICON_BUTTON_SIZE_SM" @click="emit('delete-node', row)">
              <Delete />
            </IconButton>
            <IconButton class="row-action" tooltip="查看详情" tone="muted" :size="ICON_BUTTON_SIZE_SM" @click="emit('show-detail', row)">
              <InfoFilled />
            </IconButton>
          </td>
        </tr>
        <tr v-if="bottomPadding > 0" class="spacer-row"><td :colspan="6" :style="{ height: `${bottomPadding}px` }"></td></tr>
      </tbody>
    </table>
    <div v-if="!loading && !nodes.length" class="table-empty">暂无节点数据</div>
    <div v-if="loadingMore" class="table-loading">正在加载更多...</div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { Aim, Delete, InfoFilled } from '@element-plus/icons-vue'
import IconButton from '@/components/IconButton.vue'
import { useOperationStore, useXrayStore } from '@/stores'
import { formatTransportDetail, getLatencyClass, formatLatency } from '@/utils/formatters'
import { NODE_ROW_HEIGHT, NODE_OVERSCAN, NODE_LOAD_MORE_THRESHOLD, NODE_VIEWPORT_HEIGHT_DEFAULT, ICON_BUTTON_SIZE_SM } from '@/constants'
import type { Node } from '@/types'

const props = defineProps<{
  nodes: Node[]
  loading: boolean
  loadingMore: boolean
  hasMore: boolean
}>()

const xrayStore = useXrayStore()
const operationStore = useOperationStore()
function isCurrent(node: Node) { return xrayStore.isRunning && xrayStore.currentNode?.id === node.id }
const speedTestRunning = computed(() => operationStore.running)

const emit = defineEmits<{
  'use-node': [node: Node]
  'speed-test-node': [node: Node]
  'delete-node': [node: Node]
  'show-detail': [node: Node]
  'load-more': []
}>()

const rowHeight = NODE_ROW_HEIGHT // 必须匹配 CSS --row-height-lg
const overscan = NODE_OVERSCAN
const scrollTop = ref(0)
const viewportHeight = ref(NODE_VIEWPORT_HEIGHT_DEFAULT)

const visibleStart = computed(() => Math.max(0, Math.floor(scrollTop.value / rowHeight) - overscan))
const visibleCount = computed(() => Math.ceil(viewportHeight.value / rowHeight) + overscan * 2)
const visibleEnd = computed(() => Math.min(props.nodes.length, visibleStart.value + visibleCount.value))
const visibleRows = computed(() => {
  const start = visibleStart.value
  const end = visibleEnd.value
  if (start >= end) return []
  return props.nodes.slice(start, end).map(node => ({
    ...node,
    transportLabel: formatTransportDetail(node)
  }))
})
const topPadding = computed(() => visibleStart.value * rowHeight)
const bottomPadding = computed(() => Math.max(0, (props.nodes.length - visibleEnd.value) * rowHeight))

function handleScroll(event: Event) {
  const el = event.target as HTMLElement
  scrollTop.value = el.scrollTop
  viewportHeight.value = el.clientHeight
  if (props.hasMore && !props.loadingMore && el.scrollTop + el.clientHeight >= el.scrollHeight - NODE_LOAD_MORE_THRESHOLD) {
    emit('load-more')
  }
}
</script>

<style scoped>
.node-table-shell{margin:0 var(--spacing-lg) var(--spacing-lg);flex:1;min-height:0;overflow:auto;border:1px solid var(--border-soft);border-radius:var(--radius-lg);background:var(--surface);}
.node-table{width:100%;table-layout:fixed;border-collapse:collapse;font-size:var(--font-xs);}
.col-name{width:25%;}.col-address{width:24%;}.col-protocol,.col-latency{width:12%;}.col-transport{width:15%;}.col-actions,.actions-col{width:128px;}
.node-table thead{position:sticky;top:0;z-index:2;background:var(--bg-panel);}
.node-table th,.node-table td{padding:0 var(--spacing-md);overflow:hidden;border-bottom:1px solid var(--border-soft);text-overflow:ellipsis;white-space:nowrap;}
.node-table th{height:var(--row-height);color:var(--text-tertiary);background:var(--bg-panel);border-bottom-color:var(--border-soft);text-align:left;font-weight:var(--weight-bold);}
.node-table td{height:var(--row-height-lg);font-weight:var(--weight-regular);}
.node-table tr:hover td { background: var(--bg-hover); }
.node-table tr.active td { background: var(--bg-active); }
.node-table tr.active { box-shadow: inset 2px 0 0 var(--primary); }
.spacer-row td { padding: 0 !important; border-bottom: none !important; }
.node-name{display:flex;align-items:center;gap:var(--spacing-sm);font-weight:var(--weight-bold);}
.node-name-text{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.node-endpoint,.node-transport,.node-latency{font-family:var(--mono);}
.node-endpoint,.node-transport{color:var(--text-secondary);font-size:var(--font-xs);}
.node-protocol{color:var(--text-primary);}
.actions-col{min-width:128px;overflow:visible!important;text-align:left;text-overflow:clip!important;white-space:nowrap;}
.row-action{margin-left:var(--spacing-sm);vertical-align:middle;}
.row-action:first-child{margin-left:0;}
.table-loading{height:var(--row-height-sm);display:flex;align-items:center;justify-content:center;color:var(--text-tertiary);}
.table-empty{display:flex;align-items:center;justify-content:center;padding:var(--spacing-lg);color:var(--text-tertiary);font-size:var(--font-md);}
</style>
