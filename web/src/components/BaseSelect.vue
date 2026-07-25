<template>
  <div ref="rootRef" class="base-select" :style="height ? { '--bs-height': height } : undefined">
    <button class="select-trigger" :class="{ open }" :disabled="disabled" @click.stop="toggle">
      <span class="select-label">{{ triggerLabel }}</span>
      <span class="chevron">▾</span>
    </button>
    <div v-if="open" class="select-menu" :class="{ 'placement-top': placement === 'top' }" :style="menuStyle">
      <template v-for="item in options" :key="item.value">
        <slot name="option" :item="item" :active="isItemActive(item.value)" :select="() => select(item.value)">
          <button
            class="select-option"
            :class="{ active: isItemActive(item.value) }"
            @click="select(item.value)"
          >
            <span v-if="multiple" class="check-mark">{{ isItemActive(item.value) ? '✓' : '' }}</span>
            {{ item.label }}
          </button>
        </slot>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts" generic="T extends string | string[]">
import { computed, ref } from 'vue'
import { useClickOutside, useSelectCoordinator } from '@/composables'

const props = withDefaults(defineProps<{
  modelValue: T
  options: Array<{ label: string; value: string }>
  placeholder?: string
  disabled?: boolean
  placement?: 'top' | 'bottom'
  menuWidth?: string
  height?: string
  multiple?: boolean
}>(), {
  placement: 'bottom'
})

const emit = defineEmits<{
  'update:modelValue': [value: T]
}>()

const open = ref(false)
const rootRef = ref<HTMLElement>()

const selectedArray = computed<string[]>(() => {
  const v = props.modelValue
  return Array.isArray(v) ? v : (v ? [v] : [])
})

function isItemActive(value: string) {
  return selectedArray.value.includes(value)
}

const triggerLabel = computed(() => {
  if (props.multiple) {
    if (selectedArray.value.length === 0) return props.placeholder
    const labels = selectedArray.value
      .map(v => props.options.find(item => item.value === v)?.label)
      .filter(Boolean)
    if (!labels.length) return props.placeholder
    return labels.join(' | ')
  }
  const current = typeof props.modelValue === 'string' ? props.modelValue : ''
  return props.options.find(item => item.value === current)?.label || props.placeholder
})

const menuStyle = computed(() => props.menuWidth ? { width: props.menuWidth, minWidth: props.menuWidth } : undefined)

function closeMenu() { open.value = false }
const { closeOthers, register } = useSelectCoordinator(closeMenu)
register()

function toggle() {
  if (props.disabled) return
  if (!open.value) closeOthers()
  open.value = !open.value
}

function select(value: string) {
  if (props.multiple) {
    const current = [...selectedArray.value]
    const idx = current.indexOf(value)
    if (idx >= 0) current.splice(idx, 1)
    else current.push(value)
    emit('update:modelValue', current as T)
  } else {
    emit('update:modelValue', value as T)
    open.value = false
  }
}

useClickOutside(rootRef, () => { open.value = false })
</script>

<style scoped>
.base-select {
  position: relative;
  width: 100%;
}

.select-trigger {
  width: 100%;
  height: var(--bs-height, var(--row-height));
  padding: 0 var(--spacing-md);
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
  background: var(--surface);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: var(--font-xs);
  font-weight: var(--weight-medium);
  cursor: pointer;
  transition: var(--el-transition-duration);
}

.select-trigger:hover:not(:disabled),
.select-trigger.open {
  border-color: var(--primary);
  background: var(--bg-hover);
  color: var(--text-primary);
}

.select-trigger:disabled {
  opacity: 0.45;
  filter: saturate(0.3);
  cursor: not-allowed;
}

.select-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chevron {
  color: var(--text-tertiary);
}

.select-menu {
  position: absolute;
  top: calc(100% + var(--spacing-xs));
  left: 0;
  z-index: 25;
  min-width: 100%;
  max-height: 240px;
  overflow-y: auto;
  padding: var(--spacing-xs);
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
  background: var(--bg-panel);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-lg);
  box-shadow: none;
}

.select-menu.placement-top {
  top: auto;
  bottom: calc(100% + var(--spacing-xs));
}

.select-option {
  height: var(--row-height-sm);
  padding: 0 var(--spacing-sm);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  text-align: left;
  font-size: var(--font-xs);
  font-weight: var(--weight-medium);
  white-space: nowrap;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.check-mark {
  width: 1em;
  color: var(--primary);
  font-weight: var(--weight-bold);
}

.select-option:hover,
.select-option.active {
  color: var(--text-primary);
  background: var(--bg-hover);
}
</style>
