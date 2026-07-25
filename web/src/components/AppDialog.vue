<template>
  <el-dialog
    :model-value="modelValue"
    :title="title"
    :width="width"
    append-to-body
    v-bind="dialogAttrs"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <slot />
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { DIALOG_WIDTH_DEFAULT } from '@/constants'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue: boolean
  title: string
  width?: string
}>(), {
  width: DIALOG_WIDTH_DEFAULT
})

const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

const attrs = useAttrs()

const dialogAttrs = computed(() => ({
  ...attrs,
  class: ['app-dialog', attrs.class]
}))
</script>
