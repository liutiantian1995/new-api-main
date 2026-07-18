import { useTranslation } from 'react-i18next'

import { formatNumber, formatQuota, formatTokens } from '@/lib/format'
import { StaticDataTable } from '@/components/data-table'

import type { TopUserRow } from '../types'

interface Props {
  rows: TopUserRow[]
  loading: boolean
}

export function TopUsersTable({ rows, loading }: Props) {
  const { t } = useTranslation()
  return (
    <StaticDataTable
      data={rows}
      getRowKey={(r) => r.user_id}
      emptyContent={loading ? t('Loading...') : t('No data')}
      columns={[
        {
          id: 'username',
          header: t('User'),
          cell: (r) => r.username || `#${r.user_id}`,
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
