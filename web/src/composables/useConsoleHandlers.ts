import { useGeoStore, useNodeStore, useProxyStore, useSettingsStore, useTunStore, useXrayConfigStore, useXrayStore } from '@/stores'
import { useActionExecutor } from './useActionExecutor'
import { handleError, msg } from '@/utils/message'
import { NO_AVAILABLE_NODE, SPEED_TEST_FAILED } from '@/constants'
import { ElMessageBox } from 'element-plus'
import type { RefreshContext } from '@/types'
import type { ModalName } from './useModalState'

interface ConsoleActionContext {
  refreshContext: RefreshContext
  refreshConsole: () => Promise<void>
  openModal: (name: ModalName) => void
}

export function useConsoleHandlers(ctx: ConsoleActionContext) {
  const { execute } = useActionExecutor()
  const { refreshConsoleAndNodes } = ctx.refreshContext
  const { refreshConsole, openModal } = ctx
  const xrayStore = useXrayStore()
  const nodeStore = useNodeStore()
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

  async function handleSpeedTest() {
    if (xrayStore.websiteSpeedTestLoading) { msg.warning('网站测速进行中'); return }
    if (!xrayStore.isRunning || !xrayStore.currentNode) {
      const current = xrayStore.currentNode
      const candidate = current
        ? nodeStore.nodes.find(n => n.id === current.id)
        : nodeStore.nodes.find(n => n.latency > 0)
      if (!candidate) { msg.warning(NO_AVAILABLE_NODE); return }
      await execute(() => nodeStore.activateNode(candidate.id), {
        refreshAfterAction: refreshConsoleAndNodes,
        errorMsg: '激活节点失败'
      })
    }
    await execute(
      async () => {
        await xrayStore.runWebsiteSpeedTest()
        await settingsStore.fetchConfigView()
      },
      {
        refreshAfterAction: refreshConsole,
        successMsg: '网站测速完成', errorMsg: SPEED_TEST_FAILED
      }
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
    handleSpeedTest,
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
