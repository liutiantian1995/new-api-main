export interface ReportStat {
  quota: number
  prompt_tokens: number
  completion_tokens: number
  cached_tokens: number
  total_tokens: number
  request_count: number
}

export interface TopChannelRow {
  channel_id: number
  channel_name: string
  prompt_tokens: number
  completion_tokens: number
  cached_tokens: number
  total_tokens: number
  quota: number
  request_count: number
}

export interface TopUserRow {
  user_id: number
  username: string
  prompt_tokens: number
  completion_tokens: number
  cached_tokens: number
  total_tokens: number
  quota: number
  request_count: number
}

export type TimeRangePreset = 'today' | '7d' | '30d'

export interface ReportFilters {
  preset: TimeRangePreset
  start_timestamp: number
  end_timestamp: number
}
