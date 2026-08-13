import { LATENCY_MEDIUM_THRESHOLD } from '@/constants'
import type { Transport } from '@/types'

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
  formatWithOptions(time, { hour: '2-digit', minute: '2-digit', second: '2-digit' })

export function formatTransportDetail(node: { transport?: Transport }): string {
  const transport = node.transport
  if (!transport) return 'TCP'
  const network = transport.network || 'TCP'
  const security = transport.security || ''
  const parts = [network]
  if (security && security !== 'auto') parts.push(security)
  return parts.join('+')
}

export function formatLatency(latency: number, testing = false): string {
  if (testing) return '...'
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

export function formatJsonError(content: string, error: unknown) {
  const message = error instanceof Error ? error.message : 'JSON 格式错误'
  const match = /position (\d+)/i.exec(message)
  if (!match) return message
  const position = Number(match[1])
  if (Number.isNaN(position) || position < 0) return message

  const prefix = content.slice(0, position)
  const line = prefix.split('\n').length
  const column = position - prefix.lastIndexOf('\n')
  return `${message} (第 ${line} 行，第 ${column} 列)`
}
