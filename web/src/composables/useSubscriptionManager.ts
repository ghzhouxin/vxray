import { ref } from 'vue'
import { useOperationStore, useSubscriptionStore } from '@/stores'
import { msg } from '@/utils/message'
import { useActionExecutor } from './useActionExecutor'
import type { Subscription, SubscriptionFormData, RefreshContext } from '@/types'

export function useSubscriptionManager(ctx: RefreshContext) {
  const subscriptionStore = useSubscriptionStore()
  const operationStore = useOperationStore()
  const { execute } = useActionExecutor()
  const { refreshConsoleAndNodes } = ctx

  const updatingSubscriptionId = ref(0)
  const batchUpdating = ref(false)
  const submitLoading = ref(false)

  async function handleSubmitSubscription(id: number | null, data: SubscriptionFormData) {
    await execute(
      async () => {
        if (id) await subscriptionStore.updateSubscription(id, data)
        else await subscriptionStore.addSubscription(data)
      },
      {
        refreshAfterAction: refreshConsoleAndNodes,
        loading: submitLoading,
        successMsg: '保存成功',
        errorMsg: '保存订阅失败'
      }
    )
  }

  async function runSubscriptionUpdate(ids?: number[]) {
    if (!ids?.length && !subscriptionStore.subscriptions.length) { msg.warning('暂无订阅'); return }
    batchUpdating.value = !ids?.length
    updatingSubscriptionId.value = ids?.[0] || 0
    operationStore.start('subscription_update', ids?.length ? '更新订阅' : '批量更新订阅')

    try {
      await subscriptionStore.refreshSubscriptions(ids, progress => operationStore.applyProgress(progress))
      const isSingle = ids?.length === 1
      const isBatch = !ids?.length
      const last = operationStore.active
      if (last?.failed && last.failed > 0) {
        if (isSingle) msg.error('订阅更新失败')
        else if (isBatch) msg.error(`全部订阅更新完成：成功 ${last.success}，失败 ${last.failed}`)
        else msg.error(`订阅更新完成：成功 ${last.success}，失败 ${last.failed}`)
      } else {
        msg.success(isSingle ? '订阅更新成功' : isBatch ? '全部订阅更新完成' : '订阅更新完成')
      }
    } catch (error) {
      msg.error(ids?.length ? '更新订阅失败' : '更新全部订阅失败')
      throw error
    } finally {
      updatingSubscriptionId.value = 0
      batchUpdating.value = false
      operationStore.clear()
      await refreshConsoleAndNodes().catch(() => undefined)
    }
  }

  function handleUpdateSubscription(id: number) { return runSubscriptionUpdate([id]) }
  function handleUpdateAllSubscriptions() { return runSubscriptionUpdate() }

  async function handleDeleteSubscription(sub: Subscription) {
    await execute(() => subscriptionStore.deleteSubscription(sub.id), {
      refreshAfterAction: refreshConsoleAndNodes,
      successMsg: '删除成功',
      errorMsg: '删除订阅失败'
    })
  }

  return {
    updatingSubscriptionId,
    batchUpdating,
    submitLoading,
    handleSubmitSubscription,
    handleUpdateSubscription,
    handleUpdateAllSubscriptions,
    handleDeleteSubscription
  }
}
