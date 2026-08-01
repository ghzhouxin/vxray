import type { Node } from './node'

export interface XrayStatusResponse {
  running: boolean
  current_node?: Node
}
