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
  amount: number
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

// 时间粒度，复用 dashboard 的同名词典：hour / day / week
export type ReportTimeGranularity = 'hour' | 'day' | 'week'

// 渠道排名展示策略：
// - 'auto'  = 默认值，由后端 total * 10% 决定（最少 10）
// - 'all'   = 全部展示
// - number  = 显式上限
export type ChannelLimitMode = 'auto' | 'all' | number

export interface ReportFilters {
  start_timestamp: number
  end_timestamp: number
  channel_limit_mode: ChannelLimitMode
}
