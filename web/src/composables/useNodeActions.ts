import { ref } from 'vue'
import { useNodeStore } from '@/stores'
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

  async function handleDeleteFailed() {
    let count = 0
    await execute(
      async () => { count = await nodeStore.deleteFailedNodes(nodeStore.activeFilter) },
      {
        refreshAfterAction: refreshConsoleAndNodes,
        loading: deletingFailedNodes,
        successMsg: () => `已删除 ${count} 个超时节点`,
        errorMsg: '删除超时节点失败'
      }
    )
  }

  return { deletingFailedNodes, handleUseNode, handleDeleteNode, handleDeleteFailed }
}
