<template>
  <div class="console-shell" @click="handleConsoleClick">
    <header class="console-header">
      <div class="brand">
        <svg class="brand-icon" width="30" height="30" viewBox="0 0 30 30" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
          <defs>
            <linearGradient id="brandBg" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stop-color="#A78BFA"/>
              <stop offset="50%" stop-color="#7186F5"/>
              <stop offset="100%" stop-color="#4DA3FF"/>
            </linearGradient>
          </defs>
          <rect width="30" height="30" rx="8" fill="url(#brandBg)"/>
          <path d="M6 5L15 14.27L24 5" stroke="rgba(255,255,255,0.22)" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M6 10L15 19.27L24 10" stroke="rgba(255,255,255,0.42)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M6 15L15 24.27L24 15" stroke="rgba(255,255,255,0.82)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
          <circle cx="6" cy="15" r="0.85" fill="rgba(255,255,255,0.82)"/>
          <circle cx="24" cy="15" r="0.85" fill="rgba(255,255,255,0.82)"/>
          <circle cx="15" cy="24.27" r="0.9" fill="rgba(255,255,255,0.86)"/>
        </svg>
        <span>VXRAY</span>
      </div>
      <div v-if="speedTestTaskStatus" class="task-status-bar">
        <span class="task-title">{{ speedTestTaskStatus.title }}</span>
        <span class="task-progress">{{ formatTaskNumber(speedTestTaskStatus.completed, speedTestTaskStatus.total) }}<span class="sep">/</span>{{ formatTaskNumber(speedTestTaskStatus.total, speedTestTaskStatus.total) }}</span>
        <span class="task-metric success">可用 {{ formatTaskNumber(speedTestTaskStatus.available, speedTestTaskStatus.total) }}</span>
        <span class="task-metric failed">超时 {{ formatTaskNumber(speedTestTaskStatus.timeout, speedTestTaskStatus.total) }}</span>
      </div>
      <div class="node-summary">
        <span>全部 <strong>{{ nodeSummary.all || '-' }}</strong></span>
        <span>可用 <strong class="success">{{ nodeSummary.available || '-' }}</strong></span>
        <span>待测 <strong class="warning">{{ nodeSummary.pending || '-' }}</strong></span>
        <span>超时 <strong class="error">{{ nodeSummary.timeout || '-' }}</strong></span>
      </div>
    </header>

    <main class="console-main">
      <ControlPanel
        :logs-visible="!logsCollapsed"
        :speed-test-disabled="autoSpeedTestPending"
        :probing="websiteProbing"
        :probe-message="websiteProbeMessage"
        @toggle-power="handlePowerToggle"
        @toggle-proxy="handleProxyToggle"
        @toggle-logs="toggleLogs"
        @speed-test="handleSpeedTest"
        @website-probe="handleWebsiteProbe"
        @open-speedtest-targets="openModal('speedTestTargets')"
        @open-subscriptions="openModal('subscriptions')"
        @open-xray-config="openXrayConfigModal"
        @open-settings="openSettingsModal"
        @open-runtime="openRuntimeModal"
      />

      <div class="work-pane">
        <NodeToolbar
        :subscription-updating="updatingAllSubscriptions"
        :delete-loading="deletingFailedNodes"
        @update-all-subscriptions="handleUpdateAllSubscriptions"
        @speed-test-retest="handleRetestTimeout"
        @speed-test-available="handleSpeedTestAvailable"
        @delete-timeout="handleDeleteFailed"
      />
        <NodeTable
          :nodes="nodeStore.nodes"
          :loading="nodeStore.loading"
          :loading-more="nodeStore.loadingMore"
          :has-more="nodeStore.hasMore"
          @use-node="handleUseNode"
          @speed-test-node="handleNodeSpeedTest"
          @delete-node="handleDeleteNode"
          @show-detail="openNodeDetail"
          @load-more="nodeStore.loadMoreNodes"
        />
      </div>
    </main>

    <LogPanel
      v-model:collapsed="logsCollapsed"
      v-model:auto-refresh="logsAutoRefresh"
      v-model:level="logFilter.level"
      v-model:tag="logFilter.tag"
      v-model:keyword="logFilter.keyword"
      :logs="logs"
      :tags="logTags"
      :levels="logLevels"
      :loading="logsLoading"
      :has-more="logHasMore"
      @clear="clearLogs"
      @load-more="loadMoreLogs"
    />

    <SubscriptionManagerDialog
      v-model="subscriptionsVisible"
      :updating-id="updatingSubscriptionId"
      :updating-all="updatingAllSubscriptions"
      :submitting="submittingSubscription"
      @submit="handleSubmitSubscription"
      @update-subscription="handleUpdateSubscription"
      @update-all="handleUpdateAllSubscriptions"
      @delete-subscription="handleDeleteSubscription"
    />

    <SystemSettingsDialog
      v-model="settingsVisible"
      @save="handleSaveUserSettings"
    />

    <SpeedTestTargetsDialog
      v-model="speedTestTargetsVisible"
      @save="handleSaveUserSettings"
    />

    <XrayConfigDialog
      v-model="xrayConfigVisible"
      @save="handleSaveXrayConfig"
      @saved="handleXrayConfigSaved"
    />

    <RuntimeDialog
      v-model="runtimeVisible"
      :system-meta="configStore.systemMeta"
      :ports="runtimePorts"
    />

    <NodeDetailDialog
      v-model="nodeDetailVisible"
      :node="selectedNode"
    />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { onKeyStroke } from '@vueuse/core'
import ControlPanel from '@/components/console/ControlPanel.vue'
import LogPanel from '@/components/console/LogPanel.vue'
import NodeDetailDialog from '@/components/console/NodeDetailDialog.vue'
import NodeTable from '@/components/console/NodeTable.vue'
import NodeToolbar from '@/components/console/NodeToolbar.vue'
import RuntimeDialog from '@/components/console/RuntimeDialog.vue'
import SpeedTestTargetsDialog from '@/components/console/SpeedTestTargetsDialog.vue'
import SubscriptionManagerDialog from '@/components/console/SubscriptionManagerDialog.vue'
import SystemSettingsDialog from '@/components/console/SystemSettingsDialog.vue'
import XrayConfigDialog from '@/components/console/XrayConfigDialog.vue'
import { useNodeStore, useSettingsStore, useXrayStore } from '@/stores'
import { useAutoRefresh, useConsoleHandlers, useConsoleLogs, useConsoleRefresh, useModalState, useNodeActions, useSpeedTest, useSubscriptionManager, useWebsiteProbe } from '@/composables'
import { STATUS_REFRESH_INTERVAL } from '@/constants'
import { handleError } from '@/utils/message'
import type { Node } from '@/types'

const xrayStore = useXrayStore()
const nodeStore = useNodeStore()
const configStore = useSettingsStore()

const {
  collapsed: logsCollapsed,
  autoRefresh: logsAutoRefresh,
  logs,
  tags: logTags,
  levels: logLevels,
  loading: logsLoading,
  hasMore: logHasMore,
  filter: logFilter,
  applySnapshot: applyLogsSnapshot,
  loadLogs,
  loadMore: loadMoreLogs,
  clearLogs,
  showLogs,
  handleConsoleClick
} = useConsoleLogs()

const {
  nodeSummary, runtimePorts,
  refreshConsole, refreshNodes, refreshConsoleAndNodes, refreshLogsSilently
} = useConsoleRefresh({ applyLogsSnapshot, loadLogs })

const refreshContext = { refreshConsoleAndNodes, refreshLogsSilently, showLogs }

const {
  speedTestTaskStatus,
  autoSpeedTestPending,
  handleRetestTimeout, handleSpeedTestAvailable, handleNodeSpeedTest,
  triggerAutoSpeedTest,
  restoreSpeedTestStatus, cleanup: cleanupSpeedTest
} = useSpeedTest(refreshContext)

const { probing: websiteProbing, probeMessage: websiteProbeMessage, runProbe: handleWebsiteProbe } = useWebsiteProbe(refreshContext)

const {
  updatingSubscriptionId, updatingAllSubscriptions, submittingSubscription,
  handleSubmitSubscription,
  handleUpdateSubscription, handleUpdateAllSubscriptions,
  handleDeleteSubscription
} = useSubscriptionManager(refreshContext)

const { deletingFailedNodes, handleUseNode, handleDeleteNode, handleDeleteFailed } = useNodeActions({
  refreshContext,
  onNodeActivated: triggerAutoSpeedTest
})

const {
  subscriptionsVisible, settingsVisible, xrayConfigVisible, runtimeVisible, nodeDetailVisible,
  speedTestTargetsVisible, openModal
} = useModalState()

const selectedNode = ref<Node | null>(null)

const {
  handlePowerToggle, handleSpeedTest, handleProxyToggle,
  openSettingsModal, openRuntimeModal, openXrayConfigModal,
  handleSaveUserSettings, handleSaveXrayConfig, handleXrayConfigSaved
} = useConsoleHandlers({ refreshContext, refreshConsole, openModal })

function formatTaskNumber(value: number, total: number) {
  if (!total) return '--'
  return String(value).padStart(String(total).length, '0')
}

function toggleLogs() {
  logsCollapsed.value = !logsCollapsed.value
}

const matchKey = (code: string) => (e: KeyboardEvent) => e.altKey && e.code === code
const preventDefault = (fn: () => void) => (e: KeyboardEvent) => { e.preventDefault(); fn() }

onKeyStroke(matchKey('KeyG'), preventDefault(toggleLogs))
onKeyStroke(matchKey('KeyT'), preventDefault(handleRetestTimeout))
onKeyStroke(matchKey('KeyR'), preventDefault(handleWebsiteProbe))
onKeyStroke(matchKey('KeyV'), preventDefault(handleSpeedTestAvailable))
onKeyStroke(matchKey('KeyD'), preventDefault(handleDeleteFailed))
onKeyStroke(matchKey('KeyS'), preventDefault(handleUpdateAllSubscriptions))

async function loadPageData() {
  await refreshConsole()
  await Promise.all([refreshNodes(), configStore.fetchConfigView()])
  await restoreSpeedTestStatus()
}

function openNodeDetail(node: Node) { selectedNode.value = node; openModal('nodeDetail') }

onMounted(() => { loadPageData().catch(e => handleError(e, '加载主页面数据失败')) })
onBeforeUnmount(() => { cleanupSpeedTest() })
useAutoRefresh(() => xrayStore.fetchStatus(), STATUS_REFRESH_INTERVAL, { onError: e => handleError(e, '刷新状态失败') })
</script>

<style scoped>
.console-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--bg-base);
  color: var(--text-primary);
  font-size: var(--font-sm);
  -webkit-font-smoothing: antialiased;
  text-rendering: geometricPrecision;
}

.console-shell,
:deep(.console-shell *) {
  scrollbar-width: none;
}

:deep(.console-shell *)::-webkit-scrollbar {
  display: none;
  width: 0;
  height: 0;
}

.console-main,
.work-pane {
  flex: 1;
  display: flex;
  min-height: 0;
}

.work-pane {
  min-width: 0;
  flex-direction: column;
}

.console-header {
  height: var(--header-height);
  padding: 0 var(--spacing-lg);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
  border-bottom: 1px solid var(--border-soft);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-lg);
  font-weight: var(--weight-bold);
  letter-spacing: -.015em;
}

.brand-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  display: block;
}

.node-summary {
  display: flex;
  gap: var(--spacing-sm);
  color: var(--text-secondary);
}

.node-summary span {
  height: var(--row-height-sm);
  padding: 0 var(--spacing-sm);
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-sm);
  background: var(--surface);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-md);
}

.node-summary strong {
  font-weight: var(--weight-bold);
  font-variant-numeric: tabular-nums;
}

.task-status-bar {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  height: var(--row-height-sm);
  padding: 0 var(--spacing-sm);
  background: var(--primary-light);
  border: 1px solid var(--border-active);
  border-radius: var(--radius-md);
  font-size: var(--font-sm);
  color: var(--text-secondary);
  font-variant-numeric: tabular-nums;
}

.task-status-bar .task-title {
  color: var(--primary);
  font-weight: var(--weight-bold);
}

.task-status-bar .task-progress {
  font-family: var(--mono);
  color: var(--text-primary);
}

.task-status-bar .task-progress .sep {
  margin: 0 var(--spacing-xs);
  color: var(--text-tertiary);
}

.task-status-bar .task-metric.success {
  color: var(--success);
}

.task-status-bar .task-metric.failed {
  color: var(--danger);
}

.node-summary .error {
  color: var(--danger) !important;
}

.node-summary .success {
  color: var(--success);
}

.node-summary .warning {
  color: var(--warning);
}
</style>
