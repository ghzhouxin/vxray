import { reactive, ref } from 'vue'

type CardEditorKey = number | 'new' | null

export function useCardEditor<TForm extends object>(initialForm: TForm) {
  const expandedKey = ref<CardEditorKey>(null)
  const form = reactive<TForm>({ ...initialForm })

  function resetForm() {
    Object.assign(form, initialForm)
  }

  function startCreate() {
    resetForm()
    expandedKey.value = 'new'
  }

  function startEdit(key: number, data: Partial<TForm>) {
    if (expandedKey.value === key) {
      cancelEdit()
      return
    }
    Object.assign(form, data)
    expandedKey.value = key
  }

  function cancelEdit() {
    expandedKey.value = null
    resetForm()
  }

  return { expandedKey, form, resetForm, startCreate, startEdit, cancelEdit }
}
