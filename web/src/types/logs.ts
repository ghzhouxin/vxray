export interface Log {
  id: number
  tag: string
  message: string
  detail: string
  created_at: string
  updated_at: string
}

export interface LogListResponse {
  items: Log[]
  tags: string[]
  levels: string[]
  has_more: boolean
  next_cursor: string
}
