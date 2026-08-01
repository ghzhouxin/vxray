import { defineStore } from 'pinia'
import { ref } from 'vue'
import { geoApi } from '@/api'
import { withLoading } from '@/utils/async'
import type { GeoStatus } from '@/types'

export const useGeoStore = defineStore('geo', () => {
  const geoStatus = ref<GeoStatus>({ geoip_exists: false, geosite_exists: false, geoip_size: 0, geosite_size: 0, geoip_modified: '', geosite_modified: '' })
  const loading = ref(false)

  async function fetchGeoStatus() {
    const data = await geoApi.getStatus()
    geoStatus.value = data
  }

  async function downloadGeoFiles() {
    await withLoading(loading, async () => {
      await geoApi.downloadAll()
      await fetchGeoStatus()
    })
  }

  return { geoStatus, loading, fetchGeoStatus, downloadGeoFiles }
})
