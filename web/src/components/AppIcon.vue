<template>
  <el-tooltip :content="name" placement="top" effect="dark" :show-after="TOOLTIP_SHOW_AFTER_MS" :disabled="!name">
    <span class="app-icon">
      <img
        v-if="src && !failed"
        :src="src"
        :alt="name"
        :style="{ width: `${size}px`, height: `${size}px` }"
        class="app-icon-img"
        @error="failed = true"
      />
      <span
        v-else
        class="app-icon-fallback"
        :style="{
          width: `${size}px`,
          height: `${size}px`,
          backgroundColor: fallbackColor
        }"
      />
      <span v-if="initials" class="app-icon-text">{{ initials }}</span>
    </span>
  </el-tooltip>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { TOOLTIP_SHOW_AFTER_MS } from '@/constants'

const PALETTE = ['#e3f2fd', '#f3e5f5', '#e8f5e9', '#fff3e0', '#fce4ec', '#e0f7fa']

const props = withDefaults(defineProps<{
  src?: string
  name?: string
  size?: number
}>(), {
  src: '',
  name: '',
  size: 16
})

const failed = ref(false)

watch(() => props.src, () => { failed.value = false })

const initials = computed(() => {
  const n = props.name
  if (!n) return ''
  if (n.length === 1) return n
  return n.charAt(0) + n.charAt(n.length - 1)
})

const fallbackColor = computed(() => {
  if (!props.name) return PALETTE[0]
  let hash = 0
  for (let i = 0; i < props.name.length; i++) {
    hash = ((hash << 5) - hash) + props.name.charCodeAt(i)
    hash |= 0
  }
  return PALETTE[Math.abs(hash) % PALETTE.length]
})
</script>

<style scoped>
.app-icon {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.app-icon-img {
  border-radius: var(--radius-xs);
  object-fit: contain;
  flex-shrink: 0;
}

.app-icon-fallback {
  display: inline-block;
  border-radius: var(--radius-xs);
  flex-shrink: 0;
}

.app-icon-text {
  font-size: var(--font-xs);
  color: var(--text-secondary);
  font-weight: var(--weight-medium);
}
</style>
