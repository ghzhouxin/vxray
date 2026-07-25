<template>
  <AppDialog :model-value="modelValue" title="节点详情" @update:model-value="emit('update:modelValue', $event)">
    <div v-if="node" class="detail-grid">
      <div><span>节点</span>{{ node.name }}</div>
      <div><span>地址</span>{{ node.address }}:{{ node.port }}</div>
      <div><span>协议</span>{{ node.protocol_label }}</div>
      <div><span>传输</span>{{ formatTransportDetail(node) }}</div>
      <div><span>延迟</span>{{ formatLatency(node.latency) }}</div>
      <div><span>订阅</span>#{{ node.subscription_id }}</div>
      <div><span>创建时间</span>{{ formatDateTime(node.created_at) }}</div>
      <div><span>更新时间</span>{{ formatDateTime(node.updated_at) }}</div>
      <div><span>原始链接</span>{{ node.raw_url || '-' }}</div>
    </div>
  </AppDialog>
</template>

<script setup lang="ts">
import { formatTransportDetail, formatDateTime, formatLatency } from '@/utils/formatters'
import type { Node } from '@/types'
import AppDialog from '@/components/AppDialog.vue'

defineProps<{ modelValue: boolean; node: Node | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
</script>
