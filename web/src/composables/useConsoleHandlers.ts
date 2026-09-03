import { useGeoStore, useProxyStore, useSettingsStore, useTunStore, useXrayConfigStore, useXrayStore } from '@/stores'
import { useActionExecutor } from './useActionExecutor'
import { handleError } from '@/utils/message'
import { ElMessageBox } from 'element-plus'
import type { RefreshContext } from '@/types'
import type { ModalName } from './useModalState'

interface ConsoleActionContext {
  refreshContext: RefreshContext
  openModal: (name: ModalName) => void
}

export function useConsoleHandlers(ctx: ConsoleActionContext) {
  const { execute } = useActionExecutor()
  const { refreshConsole, refreshConsoleAndNodes } = ctx.refreshContext
  const { openModal } = ctx
  const xrayStore = useXrayStore()
  const proxyStore = useProxyStore()
  const tunStore = useTunStore()
  const settingsStore = useSettingsStore()
  const xrayConfigStore = useXrayConfigStore()
  const geoStore = useGeoStore()

  async function handlePowerToggle() {
    const stopping = xrayStore.isRunning
    await execute(
      async () => { if (stopping) await xrayStore.stopXray(); else await xrayStore.startXray() },
      { refreshAfterAction: refreshConsoleAndNodes, errorMsg: stopping ? '停止失败' : '启动失败' }
    )
  }

  async function handleProxyToggle() {
    const willEnable = !proxyStore.systemProxyEnabled
    await execute(() => proxyStore.toggleProxy(), {
      refreshAfterAction: refreshConsoleAndNodes,
      successMsg: willEnable ? '系统代理已开启' : '系统代理已关闭',
      errorMsg: '切换系统代理失败'
    })
  }

  async function handleTunToggle() {
    const willEnable = !tunStore.isEnabled
    await execute(
      async () => { if (willEnable) await tunStore.enable(); else await tunStore.disable() },
      {
        refreshAfterAction: refreshConsoleAndNodes,
        successMsg: willEnable ? 'TUN 模式已开启' : 'TUN 模式已关闭',
        errorMsg: '切换 TUN 模式失败'
      }
    )
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
    await execute(() => settingsStore.saveUserSettings(), {
      refreshAfterAction: refreshConsole,
      successMsg: '设置已保存',
      errorMsg: '保存失败'
    })
  }
  async function handleSaveXrayConfig() {
    await execute(() => xrayConfigStore.saveXrayConfig(), {
      refreshAfterAction: refreshConsole,
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
      { refreshAfterAction: refreshConsoleAndNodes, successMsg: 'Xray 已重启', errorMsg: '重启失败' }
    )
  }

  return {
    handlePowerToggle,
    handleProxyToggle,
    handleTunToggle,
    openSettingsModal,
    openRuntimeModal,
    openXrayConfigModal,
    handleSaveUserSettings,
    handleSaveXrayConfig,
    handleXrayConfigSaved
  }
}
