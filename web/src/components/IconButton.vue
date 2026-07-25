<template>
  <el-tooltip v-if="tooltip" :content="tooltip" placement="top" effect="dark" :show-after="TOOLTIP_SHOW_AFTER_MS">
    <button v-bind="resolvedButtonAttrs">
      <el-icon><slot /></el-icon>
      <span v-if="label" class="ib-label">{{ label }}</span>
    </button>
  </el-tooltip>
  <button v-else v-bind="resolvedButtonAttrs">
    <el-icon><slot /></el-icon>
    <span v-if="label" class="ib-label">{{ label }}</span>
  </button>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { TOOLTIP_SHOW_AFTER_MS } from '@/constants'

defineOptions({ inheritAttrs: false })

type ButtonTone = 'default' | 'primary' | 'danger' | 'success' | 'muted'

const props = withDefaults(defineProps<{
  tooltip?: string
  tone?: ButtonTone
  size?: string
  label?: string
  block?: boolean
  working?: boolean
}>(), {
  tone: 'default'
})

const emit = defineEmits<{ click: [] }>()

const attrs = useAttrs()

const toneStyles: Record<ButtonTone, Record<string, string>> = {
  default: {},
  primary: { '--ib-color': 'var(--primary)', '--ib-hover-bg': 'var(--primary-light)' },
  danger: { '--ib-color': 'var(--danger)', '--ib-hover-bg': 'var(--danger-light)' },
  success: { '--ib-color': 'var(--success)', '--ib-hover-bg': 'var(--success-light)' },
  muted: { '--ib-color': 'var(--text-secondary)', '--ib-hover-bg': 'var(--bg-hover)' }
}

const buttonStyle = computed(() => {
  const style: Record<string, string> = {}
  if (props.size) style['--ib-size'] = props.size
  Object.assign(style, toneStyles[props.tone])
  return style
})

const resolvedButtonAttrs = computed(() => ({
  ...attrs,
  class: ['icon-button', attrs.class, {'has-label': !!props.label, block: props.block, working: props.working }],
  style: [attrs.style, buttonStyle.value],
  onClick: () => emit('click')
}))
</script>

<style scoped>
.icon-button {
  width: var(--ib-size, var(--row-height));
  height: var(--ib-size, var(--row-height));
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  color: var(--ib-color, var(--text-primary));
  font-size: calc(var(--ib-size, var(--row-height)) * 0.5);
  border: 1px solid transparent;
  border-radius: calc(var(--ib-size, var(--row-height)) * 0.24);
  cursor: pointer;
  transition: var(--el-transition-duration);
}

.icon-button.has-label {
  width: auto;
  gap: calc(var(--ib-size, var(--row-height)) * 0.12);
  padding: 0 calc(var(--ib-size, var(--row-height)) * 0.4);
  font-size: calc(var(--ib-size, var(--row-height)) * 0.42);
}

.icon-button.block {
  width: 100%;
  justify-content: flex-start;
}

.icon-button:hover:not(:disabled) {
  background: var(--ib-hover-bg, var(--bg-hover));
}

.icon-button:disabled {
  opacity: 0.45;
  filter: saturate(0.3);
  cursor: not-allowed;
}

.icon-button.working :deep(svg) {
  animation: pulse 1.2s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: .62; transform: scale(.92); }
}
</style>
