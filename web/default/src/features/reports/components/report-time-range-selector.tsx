import { useTranslation } from 'react-i18next'
import { useMemo } from 'react'

import { Button } from '@/components/ui/button'
import type { TimeRangePreset } from '../types'

const PRESETS: { value: TimeRangePreset; key: string }[] = [
  { value: 'today', key: 'Today' },
  { value: '7d', key: 'Last 7 days' },
  { value: '30d', key: 'Last 30 days' },
]

interface Props {
  preset: TimeRangePreset
  onPresetChange: (preset: TimeRangePreset) => void
}

export function ReportTimeRangeSelector({ preset, onPresetChange }: Props) {
  const { t } = useTranslation()
  const items = useMemo(
    () => PRESETS.map((p) => ({ ...p, label: t(p.key) })),
    [t]
  )
  return (
    <div className='flex flex-wrap gap-2'>
      {items.map((p) => (
        <Button
          key={p.value}
          type='button'
          variant={preset === p.value ? 'default' : 'outline'}
          size='sm'
          onClick={() => onPresetChange(p.value)}
        >
          {p.label}
        </Button>
      ))}
    </div>
  )
}
