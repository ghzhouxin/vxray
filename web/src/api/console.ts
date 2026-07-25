import request from '@/utils/request'
import type { ConsoleSnapshot } from '@/types'

export const consoleApi = {
  get: () => request.get<ConsoleSnapshot>('/console')
}
