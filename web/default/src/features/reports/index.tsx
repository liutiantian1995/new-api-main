import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { ReportStatCards } from './components/report-stat-cards'
import { ReportTimeRangeSelector } from './components/report-time-range-selector'
import { TopChannelsTable } from './components/top-channels-table'
import { TopUsersTable } from './components/top-users-table'
import { getReportStats, getTopChannels, getTopUsers } from './api'
import type { TimeRangePreset } from './types'

function computeRange(preset: TimeRangePreset): {
  start_timestamp: number
  end_timestamp: number
} {
  const now = Math.floor(Date.now() / 1000)
  const day = 24 * 3600
  switch (preset) {
    case 'today': {
      const startOfDay = new Date()
      startOfDay.setHours(0, 0, 0, 0)
      return {
        start_timestamp: Math.floor(startOfDay.getTime() / 1000),
        end_timestamp: now,
      }
    }
    case '7d':
      return { start_timestamp: now - 7 * day, end_timestamp: now }
    case '30d':
    default:
      return { start_timestamp: now - 30 * day, end_timestamp: now }
  }
}

export function ReportsPage() {
  const { t } = useTranslation()
  const [preset, setPreset] = useState<TimeRangePreset>('7d')
  const range = useMemo(() => computeRange(preset), [preset])

  const statsQuery = useQuery({
    queryKey: ['reports', 'stats', preset],
    queryFn: () => getReportStats(range),
    staleTime: 60 * 1000,
  })

  const topChannelsQuery = useQuery({
    queryKey: ['reports', 'top-channels', preset],
    queryFn: () => getTopChannels({ ...range, limit: 10 }),
    staleTime: 60 * 1000,
  })

  const topUsersQuery = useQuery({
    queryKey: ['reports', 'top-users', preset],
    queryFn: () => getTopUsers({ ...range, limit: 10 }),
    staleTime: 60 * 1000,
  })

  return (
    <div className='mx-auto w-full max-w-7xl space-y-6 p-4 sm:p-6'>
      <div className='flex flex-col gap-1'>
        <h1 className='text-2xl font-semibold tracking-tight'>
          {t('Reports')}
        </h1>
        <p className='text-sm text-muted-foreground'>
          {t(
            'Token / request / quota consumption across channels and users within a time range.'
          )}
        </p>
      </div>

      <ReportTimeRangeSelector preset={preset} onPresetChange={setPreset} />

      <ReportStatCards stat={statsQuery.data} loading={statsQuery.isLoading} />

      <section className='space-y-2'>
        <h2 className='text-lg font-semibold'>{t('Top Channels')}</h2>
        <TopChannelsTable
          rows={topChannelsQuery.data ?? []}
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
