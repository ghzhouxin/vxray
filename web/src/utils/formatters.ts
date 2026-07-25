import { LATENCY_MEDIUM_THRESHOLD } from '@/constants'
import type { OutboundConfig, StreamSettings } from '@/types'

export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatWithOptions(time?: string, options?: Intl.DateTimeFormatOptions): string {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN', options)
}

export const formatTime = (time?: string) =>
  formatWithOptions(time, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })

export const formatDateTime = (time?: string) => formatWithOptions(time)

export const formatClock = (time?: string) =>
  formatWithOptions(time, { hour: '2-digit', minute: '2-digit' })

export function formatTransportDetail(node: { outbound_config?: OutboundConfig }): string {
  const streamSettings: StreamSettings | undefined = node.outbound_config?.streamSettings
  if (!streamSettings) return 'TCP'
  const network = (streamSettings.network as string) || 'TCP'
  const security = (streamSettings.security as string) || ''
  const parts = [network]
  if (security && security !== 'auto') parts.push(security)
  return parts.join('+')
}

export function formatLatency(latency: number, testing = false): string {
  if (testing && !latency) return '...'
  if (latency === 0) return '-'
  return `${latency}ms`
}

export function getLatencyClass(latency: number, hasError = false): string {
  if (latency < 0) return 'latency-timeout'
  if (latency === 0) return hasError ? 'latency-error' : 'latency-pending'
  if (latency < LATENCY_MEDIUM_THRESHOLD) return 'latency-fast'
  return 'latency-slow'
}

export function formatJsonString(str: string, fallback?: string): string {
  try { return JSON.stringify(JSON.parse(str), null, 2) }
  catch { return fallback ?? str }
}
