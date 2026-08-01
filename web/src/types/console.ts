import type { LogListResponse } from './logs'
import type { Node, ProtocolOption } from './node'
import type { Subscription } from './subscription'
import type { TunStatusState } from './tun'

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
    tun_enabled: boolean
    tun_state: TunStatusState
    proxy: {
      enabled: boolean
      http_port: number
      socks_port: number
    }
    current_node?: Node
  }
  subscriptions: Subscription[]
  protocols: ProtocolOption[]
  logs: LogListResponse
}
