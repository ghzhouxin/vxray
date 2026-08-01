export type TunStatusState = 'disabled' | 'transitioning' | 'enabled' | 'unknown'

export interface TunStatusResponse {
  enabled: boolean
  state: TunStatusState
}
