import { useGeoStore, useNodeStore, useProxyStore, useSettingsStore, useXrayConfigStore, useXrayStore } from '@/stores'
import { useActionExecutor } from './useActionExecutor'
import { handleError, msg } from '@/utils/message'
import { waitForProxyReady } from '@/utils/async'
import { ElMessageBox } from 'element-plus'
import type { RefreshContext } from '@/types'
import type { ModalName } from './useModalState'

interface ConsoleActionContext {
  refreshContext: RefreshContext
  refreshConsole: () => Promise<void>
  openModal: (name: ModalName) => void
}

export function useConsoleHandlers(ctx: ConsoleActionContext) {
  const { execute } = useActionExecutor(ctx.refreshContext)
  const { refreshConsoleAndNodes } = ctx.refreshContext
  const { refreshConsole, openModal } = ctx
  const xrayStore = useXrayStore()
  const nodeStore = useNodeStore()
  const proxyStore = useProxyStore()
  const configStore = useSettingsStore()
  const xrayConfigStore = useXrayConfigStore()
  const geoStore = useGeoStore()

  async function handlePowerToggle() {
    const stopping = xrayStore.isRunning
    await execute(
      async () => { if (stopping) await xrayStore.stopXray(); else await xrayStore.startXray() },
      { refreshAfterAction: refreshConsoleAndNodes, showLogsBefore: true, errorMsg: stopping ? '停止失败' : '启动失败' }
    )
  }

  async function handleSpeedTest() {
    if (xrayStore.speedTesting) { msg.warning('网站测速进行中'); return }
    if (!xrayStore.isRunning || !xrayStore.currentNode) {
      const candidate = xrayStore.currentNode
        ? nodeStore.nodes.find(n => n.id === xrayStore.currentNode!.id)
        : nodeStore.nodes.find(n => n.latency > 0)
      if (!candidate) { msg.warning('暂无可用节点'); return }
      await execute(() => nodeStore.activateNode(candidate.id), {
        refreshAfterAction: refreshConsoleAndNodes, showLogsBefore: true,
        errorMsg: '激活节点失败'
      })
      await waitForProxyReady()
    }
    await execute(() => xrayStore.runSpeedTestMulti(), {
      refreshAfterAction: refreshConsole, showLogsBefore: true,
      successMsg: '网站测速完成', errorMsg: '测速失败'
    })
  }

  async function handleProxyToggle() {
    const willEnable = !proxyStore.systemProxyEnabled
    await execute(() => proxyStore.toggleProxy(), {
      refreshAfterAction: refreshConsoleAndNodes,
      showLogsBefore: true,
      successMsg: willEnable ? '系统代理已开启' : '系统代理已关闭',
      errorMsg: '切换系统代理失败'
    })
  }

  async function openSettingsModal() {
    openModal('settings')
    try { await geoStore.fetchGeoStatus() } catch (e) { handleError(e, '加载系统设置失败') }
  }
  async function openRuntimeModal() { openModal('runtime') }
  async function openXrayConfigModal() {
    openModal('xrayConfig')
    if (!xrayConfigStore.xrayConfigText) {
      try { await xrayConfigStore.fetchXrayConfig() }
      catch (e) { handleError(e, '加载 Xray 配置失败') }
    }
  }

  async function handleSaveUserSettings() {
    await execute(() => configStore.saveUserSettings(), {
      refreshAfterAction: refreshConsole,
      skipLogsRefresh: true,
      successMsg: '设置已保存',
      errorMsg: '保存失败'
    })
  }
  async function handleSaveXrayConfig() {
    await execute(() => xrayConfigStore.saveXrayConfig(), {
      refreshAfterAction: refreshConsole,
      showLogsBefore: true,
      successMsg: '配置已保存',
      errorMsg: '保存失败'
    })
  }

  async function handleXrayConfigSaved() {
    if (!xrayStore.isRunning) return
    try {
      await ElMessageBox.confirm('配置已保存，是否立即重启 Xray 生效？', '重启确认', {
        confirmButtonText: '重启',
        cancelButtonText: '稍后',
        type: 'success'
      })
    } catch {
      return
    }
    await execute(
      async () => { await xrayStore.stopXray(); await xrayStore.startXray() },
      { refreshAfterAction: refreshConsoleAndNodes, showLogsBefore: true, successMsg: 'Xray 已重启', errorMsg: '重启失败' }
    )
  }

  return {
    handlePowerToggle,
    handleSpeedTest,
    handleProxyToggle,
    openSettingsModal,
    openRuntimeModal,
    openXrayConfigModal,
    handleSaveUserSettings,
    handleSaveXrayConfig,
    handleXrayConfigSaved
  }
}
