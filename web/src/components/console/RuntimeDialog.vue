<template>
  <AppDialog :model-value="modelValue" title="运行信息" @update:model-value="emit('update:modelValue', $event)">
    <div class="detail-grid">
      <div><span>面板地址</span>{{ systemMeta.server.host }}:{{ systemMeta.server.port || '-' }}</div>
      <div><span>HTTP 入站</span>{{ ports.http ? `127.0.0.1:${ports.http}` : '-' }}</div>
      <div><span>SOCKS 入站</span>{{ ports.socks ? `127.0.0.1:${ports.socks}` : '-' }}</div>
      <div><span>工作目录</span>{{ systemMeta.home || '-' }}</div>
      <div><span>Xray 程序</span>{{ systemMeta.xray.binary || '-' }}</div>
      <div><span>Xray 配置</span>{{ systemMeta.paths.xray_config_path || '-' }}</div>
      <div><span>Geo 目录</span>{{ systemMeta.paths.geo_dir || '-' }}</div>
    </div>
  </AppDialog>
</template>

<script setup lang="ts">
import type { SystemMeta } from '@/types'
import AppDialog from '@/components/AppDialog.vue'

defineProps<{ modelValue: boolean; systemMeta: SystemMeta; ports: { http: number; socks: number } }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
</script>
