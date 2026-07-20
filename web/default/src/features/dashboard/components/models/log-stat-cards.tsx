/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { getUserQuotaDates } from '@/features/dashboard/api'
import { useModelStatCardsConfig } from '@/features/dashboard/hooks/use-dashboard-config'
import {
  buildQueryParams,
  calculateDashboardStats,
  getDefaultDays,
} from '@/features/dashboard/lib'
import type {
  QuotaDataItem,
  DashboardFilters,
} from '@/features/dashboard/types'
import { api } from '@/lib/api'
import {
  formatCompactNumber,
  formatNumber,
  formatQuota,
  formatTokens,
} from '@/lib/format'
import { computeTimeRange } from '@/lib/time'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

interface LogStatCardsProps {
  filters?: DashboardFilters
  onDataUpdate?: (data: QuotaDataItem[], loading: boolean) => void
}

const MAX_INLINE_STAT_CHARS = 9

// Token 类字段统一使用 K/M 紧凑格式，与 Reports 页面保持一致。
// 非 token 字段（请求数 / RPM / 配额）保留原有自适应格式。
const TOKEN_STAT_KEYS = new Set([
  'inputTokens',
  'cachedTokens',
  'outputTokens',
  'tokens',
  'avgTpm',
])

interface LogStat {
  quota: number
  rpm: number
  tpm: number
  prompt_tokens: number
  completion_tokens: number
  cached_tokens: number
  total_tokens: number
  request_count: number
}

function formatStatNumber(value: number, locale: Intl.LocalesArgument) {
  const fullValue = formatNumber(value, locale)
  const displayValue =
    fullValue.length > MAX_INLINE_STAT_CHARS
      ? formatCompactNumber(value, locale)
      : fullValue

  return {
    displayValue,
    fullValue,
  }
}

export function LogStatCards(props: LogStatCardsProps) {
  const { i18n } = useTranslation()
  const statCardsConfig = useModelStatCardsConfig()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = !!(user?.role && user.role >= 10)
  const [stats, setStats] = useState<LogStat | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  const [timeRangeMinutes, setTimeRangeMinutes] = useState(0)

  const { filters, onDataUpdate } = props

  useEffect(() => {
    const abortController = new AbortController()
    setLoading(true)

    setError(false)
    onDataUpdate?.([], true)

    const timeRange = computeTimeRange(
      getDefaultDays(filters?.time_granularity),
      filters?.start_timestamp,
      filters?.end_timestamp
    )
    const timeDiff = (timeRange.end_timestamp - timeRange.start_timestamp) / 60
    setTimeRangeMinutes(timeDiff)

    const params = buildQueryParams(timeRange, filters)
    type StatResponse = { success: boolean; data: LogStat }
    type QuotaDataResponse = { success: boolean; data: QuotaDataItem[] }

    const promise: Promise<StatResponse | QuotaDataResponse> = isAdmin
      ? api
          .get<StatResponse>('/api/log/stat', { params })
          .then((r) => r.data)
      : getUserQuotaDates(params, false)

    // 管理员路径额外并行拉取图表数据（/api/log/stat 仅返回标量统计，
    // 不携带 per-model 时序数组，模型调用分析/消耗分布图表会因此空白）
    const chartDataPromise: Promise<QuotaDataResponse | null> = isAdmin
      ? getUserQuotaDates(params, true)
          .then((data) => data)
          .catch(() => null)
      : Promise.resolve(null)

    promise
      .then((data: StatResponse | QuotaDataResponse) => {
        if (abortController.signal.aborted) return
        const payload = (data as QuotaDataResponse)?.data
        if (isAdmin && !Array.isArray(payload)) {
          setStats(payload as LogStat)
          // 等待图表数据就绪后再传递，避免先用空数组覆盖再被刷新
          chartDataPromise.then((chartRes) => {
            if (abortController.signal.aborted) return
            const chartPayload = chartRes?.data
            onDataUpdate?.(
              Array.isArray(chartPayload) ? chartPayload : [],
              false
            )
          })
        } else if (Array.isArray(payload)) {
          const c = calculateDashboardStats(payload)
          setStats({
            quota: c.totalQuota,
            rpm: c.totalCount,
            tpm: c.totalTokens,
            prompt_tokens: c.totalTokens,
            completion_tokens: 0,
            cached_tokens: 0,
            total_tokens: c.totalTokens,
            request_count: c.totalCount,
          })
          onDataUpdate?.(payload, false)
        }
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        setStats(null)
        setError(true)
        onDataUpdate?.([], false)
      })
      .finally(() => {
        if (!abortController.signal.aborted) {
          setLoading(false)
        }
      })

    return () => {
      abortController.abort()
    }
  }, [filters, isAdmin, onDataUpdate])

  const adaptedStats = {
    rpm: stats?.rpm ?? 0,
    quota: stats?.quota ?? 0,
    tpm: stats?.total_tokens ?? 0,
    prompt_tokens: stats?.prompt_tokens ?? 0,
    completion_tokens: stats?.completion_tokens ?? 0,
    cached_tokens: stats?.cached_tokens ?? 0,
    // 卡片配置 (use-dashboard-config.tsx) 直接读这两个字段：
    //   - Total Count 卡片读 stat.request_count
    //   - Total Tokens 卡片读 stat.total_tokens
    //   - Average RPM/TPM 卡片读 stat.request_count / stat.total_tokens
    // admin 路径下 stats 含真实窗口值；非 admin 路径下 LogStat 已被
    // setStats({ request_count: c.totalCount, total_tokens: c.totalTokens, ... })
    // 正确填充，所以直接透传即可。
    request_count: stats?.request_count ?? stats?.rpm ?? 0,
    total_tokens: stats?.total_tokens ?? stats?.tpm ?? 0,
  }

  const items = statCardsConfig.map((config) => {
    const rawValue = config.getValue(adaptedStats, timeRangeMinutes)
    const locale = i18n.resolvedLanguage || i18n.language
    const formatted =
      config.key === 'quota'
        ? {
            displayValue: formatQuota(rawValue),
            fullValue: formatQuota(rawValue),
          }
        : TOKEN_STAT_KEYS.has(config.key)
          ? {
              displayValue: formatTokens(rawValue),
              fullValue: formatNumber(rawValue, locale),
            }
          : formatStatNumber(rawValue, locale)

    return {
      title: config.title,
      value: formatted.displayValue,
      fullValue: formatted.fullValue,
      desc: config.description,
      icon: config.icon,
    }
  })

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid min-w-0 grid-cols-2 divide-x sm:grid-cols-4 lg:grid-cols-4 xl:grid-cols-8'>
        {items.map((it, idx) => {
          const Icon = it.icon
          return (
            <div
              key={it.title}
              className={cn(
                'min-w-0 px-3 py-2.5 sm:px-5 sm:py-4',
                idx === items.length - 1 &&
                  items.length % 2 !== 0 &&
                  'col-span-2 sm:col-span-1'
              )}
            >
              <div className='flex min-w-0 items-center gap-2'>
                <Icon className='text-muted-foreground/60 size-3.5 shrink-0' />
                <div className='text-muted-foreground truncate text-xs font-medium tracking-wider uppercase'>
                  {it.title}
                </div>
              </div>

              {loading ? (
                <div className='mt-2 flex flex-col gap-1.5'>
                  <Skeleton className='h-7 w-20' />
                  <Skeleton className='h-3.5 w-28' />
                </div>
              ) : error ? (
                <>
                  <div className='text-muted-foreground mt-1.5 font-mono text-lg font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl'>
                    --
                  </div>
                  <div className='text-muted-foreground/40 mt-1 hidden text-xs md:block'>
                    {it.desc}
                  </div>
                </>
              ) : (
                <>
                  <div
                    className='text-foreground mt-1.5 max-w-full truncate font-mono text-lg font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl'
                    title={it.fullValue}
                  >
                    {it.value}
                  </div>
                  <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
                    {it.desc}
                  </div>
                </>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
