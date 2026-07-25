import { ref } from 'vue'
import { useSubscriptionStore } from '@/stores'
import { handleError, msg } from '@/utils/message'
import { withLoading } from '@/utils/async'
import { useActionExecutor } from './useActionExecutor'
import type { Subscription, SubscriptionFormData, RefreshContext } from '@/types'

export function useSubscriptionManager(ctx: RefreshContext) {
  const subscriptionStore = useSubscriptionStore()
  const { execute } = useActionExecutor(ctx)
  const { refreshConsoleAndNodes } = ctx

  const updatingSubscriptionId = ref(0)
  const updatingAllSubscriptions = ref(false)
  const submittingSubscription = ref(false)

  async function handleSubmitSubscription(id: number | null, data: SubscriptionFormData) {
    try {
      await withLoading(submittingSubscription, async () => {
        if (id) await subscriptionStore.updateSubscription(id, data)
        else await subscriptionStore.addSubscription(data)
        await refreshConsoleAndNodes()
        msg.success('保存成功')
      })
    } catch (e) {
      handleError(e, '保存订阅失败')
    }
  }

  async function runSubscriptionUpdate(ids?: number[]) {
    if (!ids?.length && !subscriptionStore.subscriptions.length) { msg.warning('暂无订阅'); return }
    updatingAllSubscriptions.value = !ids?.length
    updatingSubscriptionId.value = ids?.[0] || 0
    await execute(
      async () => {
        const result = await subscriptionStore.refreshSubscriptions(ids)
        const isSingle = ids?.length === 1
        const isBatch = !ids?.length
        if (result.failed > 0) {
          if (isSingle) msg.error('订阅更新失败')
          else if (isBatch) msg.error(`全部订阅更新完成：成功 ${result.success}，失败 ${result.failed}`)
          else msg.error(`订阅更新完成：成功 ${result.success}，失败 ${result.failed}`)
        } else {
          msg.success(isSingle ? '订阅更新成功' : isBatch ? '全部订阅更新完成' : '订阅更新完成')
        }
      },
      { refreshAfterAction: refreshConsoleAndNodes, showLogsBefore: true, errorMsg: ids?.length ? '更新订阅失败' : '更新全部订阅失败' }
    )
    updatingSubscriptionId.value = 0
    updatingAllSubscriptions.value = false
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
    updatingAllSubscriptions,
    submittingSubscription,
    handleSubmitSubscription,
    handleUpdateSubscription,
    handleUpdateAllSubscriptions,
    handleDeleteSubscription
  }
}
