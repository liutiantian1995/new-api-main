import { useTranslation } from 'react-i18next'

import { formatNumber, formatQuota, formatTokens } from '@/lib/format'
import { StaticDataTable } from '@/components/data-table'

import type { TopChannelRow } from '../types'

interface Props {
  rows: TopChannelRow[]
  loading: boolean
}

export function TopChannelsTable({ rows, loading }: Props) {
  const { t } = useTranslation()
  return (
    <StaticDataTable
      data={rows}
      getRowKey={(r) => r.channel_id}
      emptyContent={loading ? t('Loading...') : t('No data')}
      columns={[
        {
          id: 'channel',
          header: t('Channel'),
          cell: (r) => r.channel_name || `#${r.channel_id}`,
        },
        {
          id: 'requests',
          header: t('Requests'),
          className: 'text-right',
          cellClassName: 'text-right font-mono tabular-nums',
          cell: (r) => formatNumber(r.request_count),
        },
        {
          id: 'input',
          header: t('Input Tokens'),
          className: 'text-right',
          cellClassName: 'text-right font-mono tabular-nums',
          cell: (r) => formatTokens(r.prompt_tokens),
        },
        {
          id: 'cache',
          header: t('Cache Hit'),
          className: 'text-right',
          cellClassName: 'text-right font-mono tabular-nums',
          cell: (r) => formatTokens(r.cached_tokens),
        },
        {
          id: 'output',
          header: t('Output Tokens'),
          className: 'text-right',
          cellClassName: 'text-right font-mono tabular-nums',
          cell: (r) => formatTokens(r.completion_tokens),
        },
        {
          id: 'total',
          header: t('Total Tokens'),
          className: 'text-right',
          cellClassName: 'text-right font-mono tabular-nums',
          cell: (r) => formatTokens(r.total_tokens),
        },
        {
          id: 'quota',
          header: t('Total Quota'),
          className: 'text-right',
          cellClassName: 'text-right font-mono tabular-nums',
          cell: (r) => formatQuota(r.quota),
        },
      ]}
    />
  )
}
