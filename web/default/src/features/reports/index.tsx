import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { ReportStatCards } from './components/report-stat-cards'
import { TopChannelsTable } from './components/top-channels-table'
import { TopUsersTable } from './components/top-users-table'
import {
  ReportFilterDialog,
  type ReportFilterState,
} from './components/report-filter-dialog'
import {
  getReportStats,
  getTopChannels,
  getTopUsers,
} from './api'
import type { ChannelLimitMode } from './types'

// Default window: today (00:00:00 local -> now), hourly granularity so the
// report opens with a focused snapshot of the current day's traffic.
// Admin can switch to wider ranges (1/7/14/29 days) via the filter dialog.
function defaultFilterState(): ReportFilterState {
  const now = new Date()
  const startOfDay = new Date(now)
  startOfDay.setHours(0, 0, 0, 0)
  return {
    startTimestamp: Math.floor(startOfDay.getTime() / 1000),
    endTimestamp: Math.floor(now.getTime() / 1000),
    granularity: 'hour',
  }
}

export function ReportsPage() {
  const { t } = useTranslation()
  const [filters, setFilters] = useState<ReportFilterState>(defaultFilterState)
  const [channelLimit, setChannelLimit] = useState<ChannelLimitMode>('auto')

  const range = useMemo(
    () => ({
      start_timestamp: filters.startTimestamp,
      end_timestamp: filters.endTimestamp,
    }),
    [filters]
  )

  const statsQuery = useQuery({
    queryKey: ['reports', 'stats', filters],
    queryFn: () => getReportStats(range),
    staleTime: 60 * 1000,
  })

  const topChannelsQuery = useQuery({
    // channelLimit 参与缓存 key：auto/all/number 切换后重新拉取
    queryKey: ['reports', 'top-channels', filters, channelLimit],
    queryFn: () => {
      // auto 模式下先拉一次默认 limit=10 拿到 total，再按需；这里统一
      // 先传 limit=0 拉全量前端切分（更简单可靠，避免双向依赖）。
      return getTopChannels({ ...range, limit: 0 })
    },
    staleTime: 60 * 1000,
  })

  // Derived visible rows: auto → top 10% (min 10); all → all; number → cap.
  const visibleChannels = useMemo(() => {
    const all = topChannelsQuery.data?.rows ?? []
    if (channelLimit === 'all') return all
    if (channelLimit === 'auto') {
      const total = topChannelsQuery.data?.total ?? all.length
      const autoLimit = Math.max(10, Math.ceil(total * 0.1))
      return all.slice(0, autoLimit)
    }
    return all.slice(0, channelLimit)
  }, [topChannelsQuery.data, channelLimit])

  const topUsersQuery = useQuery({
    queryKey: ['reports', 'top-users', filters],
    queryFn: () => getTopUsers({ ...range, limit: 10 }),
    staleTime: 60 * 1000,
  })

  return (
    <div className='mx-auto w-full max-w-7xl space-y-6 p-4 sm:p-6'>
      <div className='flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('Reports')}
          </h1>
          <p className='text-sm text-muted-foreground'>
            {t(
              'Token / request / quota consumption across channels and users within a time range.'
            )}
          </p>
        </div>
        <ReportFilterDialog
          current={filters}
          onApply={setFilters}
          onReset={() => setFilters(defaultFilterState())}
        />
      </div>

      <ReportStatCards stat={statsQuery.data} loading={statsQuery.isLoading} />

      <section className='space-y-2'>
        <div className='flex items-center justify-between'>
          <h2 className='text-lg font-semibold'>{t('Top Channels')}</h2>
          <ChannelLimitToggle
            mode={channelLimit}
            onChange={setChannelLimit}
          />
        </div>
        <TopChannelsTable
          rows={visibleChannels}
          loading={topChannelsQuery.isLoading}
        />
      </section>

      <section className='space-y-2'>
        <h2 className='text-lg font-semibold'>{t('Top Users')}</h2>
        <TopUsersTable
          rows={topUsersQuery.data ?? []}
          loading={topUsersQuery.isLoading}
        />
      </section>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Inline toggle for the channel limit strategy.
// Options: auto (10%), all, or explicit numeric caps (10/25/50/100).
// ---------------------------------------------------------------------------
const CHANNEL_LIMIT_OPTIONS: { mode: ChannelLimitMode; labelKey: string }[] = [
  { mode: 'auto', labelKey: 'Default (10%)' },
  { mode: 10, labelKey: 'Top 10' },
  { mode: 25, labelKey: 'Top 25' },
  { mode: 50, labelKey: 'Top 50' },
  { mode: 100, labelKey: 'Top 100' },
  { mode: 'all', labelKey: 'Show All' },
]

function ChannelLimitToggle({
  mode,
  onChange,
}: {
  mode: ChannelLimitMode
  onChange: (m: ChannelLimitMode) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-wrap gap-1'>
      {CHANNEL_LIMIT_OPTIONS.map((opt) => (
        <button
          key={String(opt.mode)}
          type='button'
          onClick={() => onChange(opt.mode)}
          className={`rounded-md border px-2 py-1 text-xs font-medium transition-colors ${
            mode === opt.mode
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-background text-muted-foreground hover:text-foreground'
          }`}
        >
          {t(opt.labelKey)}
        </button>
      ))}
    </div>
  )
}
