import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ArrowDownToLine, ArrowUpFromLine, Hash, Layers } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { formatNumber } from '@/lib/format'

import type { ReportStat } from '../types'

interface Props {
  stat: ReportStat | undefined
  loading: boolean
}

interface Item {
  key: keyof ReportStat
  labelKey: string
  icon: typeof Hash
}

const ITEMS: Item[] = [
  { key: 'request_count', labelKey: 'Total Count', icon: Hash },
  { key: 'prompt_tokens', labelKey: 'Input Tokens', icon: ArrowDownToLine },
  { key: 'cached_tokens', labelKey: 'Cache Hit', icon: Layers },
  { key: 'completion_tokens', labelKey: 'Output Tokens', icon: ArrowUpFromLine },
  { key: 'total_tokens', labelKey: 'Total Tokens', icon: Layers },
]

export function ReportStatCards({ stat, loading }: Props) {
  const { t } = useTranslation()
  const values: ReportStat = stat ?? {
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    cached_tokens: 0,
    total_tokens: 0,
    request_count: 0,
  }
  return (
    <div className='grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5'>
      {ITEMS.map((item) => {
        const Icon = item.icon
        return (
          <Card key={item.key}>
            <CardHeader className='pb-2'>
              <CardTitle className='flex items-center gap-2 text-xs font-medium tracking-wider uppercase text-muted-foreground'>
                <Icon className='size-3.5 shrink-0' />
                {t(item.labelKey)}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div
                className='truncate font-mono text-xl font-bold tracking-tight tabular-nums'
                title={formatNumber(values[item.key])}
              >
                {loading ? '—' : formatNumber(values[item.key])}
              </div>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}
