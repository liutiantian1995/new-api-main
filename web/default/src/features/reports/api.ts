import { api } from '@/lib/api'

import type {
  ReportStat,
  TopChannelRow,
  TopUserRow,
} from './types'

interface ReportStatsParams {
  start_timestamp: number
  end_timestamp: number
  channel_ids?: number[]
  user_ids?: number[]
  group?: string
}

interface TopParams {
  start_timestamp: number
  end_timestamp: number
  limit?: number
}

interface TopChannelsEnvelope {
  rows: TopChannelRow[]
  total: number
}

export async function getReportStats(
  params: ReportStatsParams
): Promise<ReportStat> {
  const query: Record<string, string | number> = {
    start_timestamp: params.start_timestamp,
    end_timestamp: params.end_timestamp,
  }
  if (params.channel_ids?.length) {
    query.channel_ids = params.channel_ids.join(',')
  }
  if (params.user_ids?.length) {
    query.user_ids = params.user_ids.join(',')
  }
  if (params.group) {
    query.group = params.group
  }
  const res = await api.get<{ success: boolean; data: ReportStat }>(
    '/api/report/stats',
    { params: query }
  )
  return res.data.data
}

export async function getTopChannels(
  params: TopParams
): Promise<TopChannelsEnvelope> {
  const res = await api.get<{
    success: boolean
    data: TopChannelsEnvelope
  }>('/api/report/top/channels', { params })
  return res.data.data ?? { rows: [], total: 0 }
}

export async function getTopUsers(
  params: TopParams
): Promise<TopUserRow[]> {
  const res = await api.get<{ success: boolean; data: TopUserRow[] }>(
    '/api/report/top/users',
    { params }
  )
  return res.data.data ?? []
}
