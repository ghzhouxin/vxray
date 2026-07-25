import type { LogListResponse } from './logs'
import type { Node, ProtocolOption } from './node'
import type { Subscription } from './subscription'
import type { WebsiteSpeedTestResult } from './xray'

export type { ProtocolOption } from './node'

export interface RefreshContext {
  refreshConsoleAndNodes: () => Promise<void>
  refreshLogsSilently: () => Promise<void>
  showLogs: () => void
}

export interface ConsoleSnapshot {
  node_summary: {
    all: number
    available: number
    pending: number
    timeout: number
  }
  runtime: {
    running: boolean
    proxy: {
      enabled: boolean
      http_port: number
      socks_port: number
    }
    ports: {
      http: number
      socks: number
    }
    current_node?: Node
  }
  speedtest_targets: WebsiteSpeedTestResult[]
  subscriptions: Subscription[]
  protocols: ProtocolOption[]
  logs: LogListResponse
}
