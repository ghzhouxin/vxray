import request from '@/utils/request'

export const proxyApi = {
  toggle: (enabled: boolean) => request.post('/proxy/toggle', { enabled })
}
