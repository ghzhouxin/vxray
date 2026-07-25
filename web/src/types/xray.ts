import type { Node } from './node'

export interface XrayStatusResponse {
  running: boolean
  current_node?: Node
}

export interface WebsiteSpeedTestResult {
  name: string
  url: string
  latency: number
  error: string
}
