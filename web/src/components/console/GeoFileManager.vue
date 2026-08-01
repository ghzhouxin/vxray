<template>
  <div class="geo-manager">
    <div class="geo-header">
      <span class="section-label">Geo 文件</span>
      <IconButton
        tooltip="更新 Geo 文件"
        :disabled="geoStore.loading"
        :working="geoStore.loading"
        tone="primary"
        :size="ICON_BUTTON_SIZE_LG"
        @click="handleDownload"
      >
        <Download />
      </IconButton>
    </div>
    <div v-if="geoStore.geoStatus.geoip_exists || geoStore.geoStatus.geosite_exists" class="geo-grid">
      <div class="geo-row" v-for="file in geoFiles" :key="file.name">
        <div class="geo-name-group">
          <span class="geo-name">{{ file.name }}</span>
          <el-tooltip :content="file.exists ? '文件存在' : '文件缺失'" placement="top" effect="dark">
            <span class="geo-status-dot" :class="{ exists: file.exists }"></span>
          </el-tooltip>
        </div>
        <span class="geo-size">{{ file.size ? formatFileSize(file.size) : '-' }}</span>
        <span class="geo-time">{{ formatTime(file.modified) || '-' }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Download } from '@element-plus/icons-vue'
import { useGeoStore } from '@/stores'
import { ICON_BUTTON_SIZE_LG } from '@/constants'
import IconButton from '@/components/IconButton.vue'
import { handleError, msg } from '@/utils/message'
import { formatFileSize, formatTime } from '@/utils/formatters'

const geoStore = useGeoStore()

const geoFiles = computed(() => [
  { name: 'geoip.dat', exists: geoStore.geoStatus.geoip_exists, size: geoStore.geoStatus.geoip_size, modified: geoStore.geoStatus.geoip_modified },
  { name: 'geosite.dat', exists: geoStore.geoStatus.geosite_exists, size: geoStore.geoStatus.geosite_size, modified: geoStore.geoStatus.geosite_modified }
])

async function handleDownload() {
  try {
    await geoStore.downloadGeoFiles()
    msg.success('Geo 文件下载成功')
  } catch (e) {
    handleError(e, '下载失败')
  }
}
</script>

<style scoped>
.geo-header { display: flex; justify-content: space-between; align-items: center; height: var(--row-height-xxs); margin-bottom: var(--spacing-sm); }
.geo-grid { display: flex; flex-direction: column; gap: var(--spacing-sm); }
.geo-row { display: grid; grid-template-columns: 1fr 60px 1fr; align-items: center; gap: var(--spacing-sm); height: var(--row-height-xxs); }
.geo-name-group { display: flex; align-items: center; gap: var(--spacing-xs); }
.geo-name { font-size: var(--font-sm); color: var(--text-tertiary); flex: 1; min-width: 0; }
.geo-status-dot { width: var(--spacing-xs); height: var(--spacing-xs); border-radius: 50%; background: var(--danger); flex-shrink: 0; cursor: pointer; }
.geo-status-dot.exists { background: var(--success); }
.geo-size { font-size: var(--font-xs); color: var(--text-tertiary); text-align: right; }
.geo-time { font-size: var(--font-xs); color: var(--text-tertiary); text-align: right; }
</style>
