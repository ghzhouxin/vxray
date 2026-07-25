import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import { geoApi } from '@/api'
import { withLoading } from '@/utils/async'
import type { GeoStatus } from '@/types'

export const useGeoStore = defineStore('geo', () => {
  const geoStatus = reactive<GeoStatus>({ geoip_exists: false, geosite_exists: false, geoip_size: 0, geosite_size: 0, geoip_modified: '', geosite_modified: '' })
  const geoDownloading = ref(false)

  async function fetchGeoStatus() {
    const data = await geoApi.getStatus()
    Object.assign(geoStatus, data)
  }

  async function downloadGeoFiles() {
    await withLoading(geoDownloading, async () => {
      await geoApi.downloadAll()
      await fetchGeoStatus()
    })
  }

  return { geoStatus, geoDownloading, fetchGeoStatus, downloadGeoFiles }
})
