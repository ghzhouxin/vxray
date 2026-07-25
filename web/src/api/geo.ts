import request from '@/utils/request'
import type { GeoStatus } from '@/types'

export const geoApi = {
  getStatus: () => request.get<GeoStatus>('/geo/status'),
  downloadAll: () => request.post('/geo/download/all')
}
