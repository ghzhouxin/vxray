import { computed, ref } from 'vue'

export type ModalName = '' | 'subscriptions' | 'settings' | 'xrayConfig' | 'runtime' | 'nodeDetail' | 'speedTestTargets'

export function useModalState() {
  const activeModal = ref<ModalName>('')

  function modalVisible(name: ModalName) {
    return computed({
      get: () => activeModal.value === name,
      set: v => { if (v) activeModal.value = name; else if (activeModal.value === name) activeModal.value = '' }
    })
  }

  function openModal(name: ModalName) { activeModal.value = name }

  const subscriptionsVisible = modalVisible('subscriptions')
  const settingsVisible = modalVisible('settings')
  const xrayConfigVisible = modalVisible('xrayConfig')
  const runtimeVisible = modalVisible('runtime')
  const nodeDetailVisible = modalVisible('nodeDetail')
  const speedTestTargetsVisible = modalVisible('speedTestTargets')

  return {
    subscriptionsVisible,
    settingsVisible,
    xrayConfigVisible,
    runtimeVisible,
    nodeDetailVisible,
    speedTestTargetsVisible,
    openModal
  }
}
