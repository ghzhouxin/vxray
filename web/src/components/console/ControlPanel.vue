<template>
  <aside class="control-pane">
    <div class="power-row">
      <IconButton
        :tooltip="xrayStore.isRunning ? '停止 Xray' : '启动 Xray'"
        :disabled="xrayStore.loading"
        data-log-keep-open
        :tone="xrayStore.isRunning ? 'success' : 'primary'"
        :size="ICON_BUTTON_SIZE_LG"
        @click="$emit('toggle-power')"
      >
        <VideoPlay v-if="!xrayStore.isRunning" />
        <VideoPause v-else />
      </IconButton>
      <IconButton
        :tooltip="`系统代理：${proxyStore.systemProxyEnabled ? '开' : '关'}`"
        :disabled="proxyStore.proxyLoading"
        :tone="proxyStore.systemProxyEnabled ? 'success' : 'default'"
        :size="ICON_BUTTON_SIZE_LG"
        data-log-keep-open
        @click="$emit('toggle-proxy')"
      >
        <Link />
      </IconButton>
      <IconButton
        class="logs-btn"
        tooltip="日志面板 (Alt+G)"
        :tone="logsVisible ? 'primary' : 'default'"
        :size="ICON_BUTTON_SIZE_LG"
        data-log-keep-open
        @click="$emit('toggle-logs')"
      >
        <Tickets />
      </IconButton>
    </div>

    <section class="panel-card">
      <div class="panel-title-row">
        <div class="title-left">
          <span>网站测速</span>
          <span v-if="probing && probeMessage" class="probe-progress">{{ probeMessage }}</span>
        </div>
        <div class="title-actions">
          <IconButton
            tooltip="管理测速网站"
            :size="ICON_BUTTON_SIZE_SM"
            tone="default"
            data-log-keep-open
            @click="$emit('open-speedtest-targets')"
          >
            <Setting />
          </IconButton>
          <IconButton
            tooltip="轮流测速 (Alt+R)"
            :size="ICON_BUTTON_SIZE_SM"
            tone="success"
            :disabled="xrayStore.speedTesting || speedTestDisabled || probing"
            :working="probing"
            data-log-keep-open
            @click="$emit('website-probe')"
          >
            <Compass />
          </IconButton>
          <IconButton
            tooltip="网站测速"
            :size="ICON_BUTTON_SIZE_SM"
            tone="success"
            :disabled="xrayStore.speedTesting || speedTestDisabled || probing"
            :working="(xrayStore.speedTesting || speedTestDisabled) && !probing"
            data-log-keep-open
            @click="$emit('speed-test')"
          >
            <Aim />
          </IconButton>
        </div>
      </div>
      <div class="speed-grid">
        <div v-for="r in speedCells" :key="r.name" class="speed-cell">
          <span>{{ r.name }}</span>
          <strong :class="r.latencyClass">{{ formatLatency(r.latency, xrayStore.speedTesting) }}</strong>
        </div>
      </div>
    </section>

    <section class="quick-actions">
      <IconButton
        block
        label="订阅管理"
        :size="ICON_BUTTON_SIZE_LG"
        tone="default"
        @click="$emit('open-subscriptions')"
      >
        <Connection />
      </IconButton>
      <IconButton
        block
        label="Xray 配置"
        :size="ICON_BUTTON_SIZE_LG"
        tone="default"
        @click="$emit('open-xray-config')"
      >
        <Document />
      </IconButton>
      <IconButton
        block
        label="系统设置"
        :size="ICON_BUTTON_SIZE_LG"
        tone="default"
        @click="$emit('open-settings')"
      >
        <Setting />
      </IconButton>
      <IconButton
        block
        label="运行信息"
        :size="ICON_BUTTON_SIZE_LG"
        tone="default"
        @click="$emit('open-runtime')"
      >
        <InfoFilled />
      </IconButton>
    </section>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Aim, Compass, Connection, Document, InfoFilled, Link, Setting, Tickets, VideoPause, VideoPlay } from '@element-plus/icons-vue'
import IconButton from '@/components/IconButton.vue'
import { useProxyStore, useXrayStore } from '@/stores'
import { ICON_BUTTON_SIZE_LG, ICON_BUTTON_SIZE_SM } from '@/constants'
import { getLatencyClass, formatLatency } from '@/utils/formatters'

const xrayStore = useXrayStore()
const proxyStore = useProxyStore()

const speedCells = computed(() => xrayStore.speedTestResults.map(r => ({ ...r, latencyClass: getLatencyClass(r.latency, !!r.error) })))

defineProps<{
  logsVisible: boolean
  speedTestDisabled?: boolean
  probing?: boolean
  probeMessage?: string
}>()

defineEmits<{
  'toggle-power': []
  'toggle-proxy': []
  'toggle-logs': []
  'speed-test': []
  'website-probe': []
  'open-speedtest-targets': []
  'open-subscriptions': []
  'open-xray-config': []
  'open-settings': []
  'open-runtime': []
}>()
</script>

<style scoped>
.control-pane {
  width: var(--control-pane-width);
  padding: var(--spacing-lg) var(--spacing-md);
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
  flex-shrink: 0;
  overflow-y: auto;
  border-right: 1px solid var(--border-soft);
}

.power-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm) var(--spacing-xs);
  flex-wrap: wrap;
}

:deep(.logs-btn) {
  margin-left: auto;
}

.panel-card {
  display: flex;
  flex-direction: column;
  padding: var(--spacing-md);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-lg);
  background: var(--surface);
}

.panel-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--spacing-xs);
}

.title-left {
  display: flex;
  align-items: baseline;
  gap: var(--spacing-xs);
}

.probe-progress {
  font-size: var(--font-xs);
  color: var(--text-tertiary);
  font-variant-numeric: tabular-nums;
}

.title-actions {
  display: flex;
  gap: var(--spacing-xs);
}

.speed-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--spacing-sm);
}

.speed-cell {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-sm);
  border-radius: var(--radius-md);
  background: var(--surface);
}

.speed-cell strong {
  font-weight: var(--weight-bold);
  font-variant-numeric: tabular-nums;
  font-family: var(--mono);
}

.quick-actions {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

@media (max-width: 980px) {
  .control-pane {
    width: 280px;
  }
}
</style>
