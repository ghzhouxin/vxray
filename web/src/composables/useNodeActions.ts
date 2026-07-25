import { ref } from 'vue'
import { useNodeStore } from '@/stores'
import { handleError, msg } from '@/utils/message'
import { withLoading } from '@/utils/async'
import { useActionExecutor } from './useActionExecutor'
import type { Node, RefreshContext } from '@/types'

interface NodeActionOptions {
  refreshContext: RefreshContext
  onNodeActivated?: () => void | Promise<void>
}

export function useNodeActions(options: NodeActionOptions) {
  const nodeStore = useNodeStore()
  const { execute } = useActionExecutor(options.refreshContext)
  const { refreshConsoleAndNodes } = options.refreshContext

  const deletingFailedNodes = ref(false)

  async function handleUseNode(node: Node) {
    const ok = await execute(() => nodeStore.activateNode(node.id), {
      refreshAfterAction: refreshConsoleAndNodes,
      showLogsBefore: true,
      successMsg: `已切换节点: ${node.name}`,
      errorMsg: '切换节点失败'
    })
    if (ok) await options.onNodeActivated?.()
  }

  async function handleDeleteNode(node: Node) {
    await execute(() => nodeStore.deleteNode(node.id), {
      refreshAfterAction: refreshConsoleAndNodes,
      successMsg: '删除成功',
      errorMsg: '删除节点失败'
    })
  }

  // 不走 execute()：useActionExecutor 未暴露 loading ref，且 execute 的 successMsg 不支持动态文案（`${count} 个`）
  async function handleDeleteFailed() {
    try {
      await withLoading(deletingFailedNodes, async () => {
        const count = await nodeStore.deleteFailedNodes(nodeStore.activeFilter)
        await refreshConsoleAndNodes()
        msg.success(`已删除 ${count} 个超时节点`)
      })
    } catch (e) {
      handleError(e, '删除超时节点失败')
    }
  }

  return { deletingFailedNodes, handleUseNode, handleDeleteNode, handleDeleteFailed }
}
