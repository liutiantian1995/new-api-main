import { Filter, RotateCcw, Calendar, Search } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { TIME_RANGE_PRESETS } from '@/features/dashboard/constants'
import { getRollingDateRange, type TimeGranularity } from '@/lib/time'
import { cn } from '@/lib/utils'

import type { ReportTimeGranularity } from '../types'

const GRANULARITY_OPTIONS: {
  value: ReportTimeGranularity
  labelKey: string
}[] = [
  { value: 'hour', labelKey: 'Hour' },
  { value: 'day', labelKey: 'Day' },
  { value: 'week', labelKey: 'Week' },
]

export interface ReportFilterState {
  startTimestamp: number
  endTimestamp: number
  granularity: ReportTimeGranularity
}

interface ReportFilterDialogProps {
  current: ReportFilterState
  onApply: (next: ReportFilterState) => void
  onReset: () => void
}

// Quick ranges imply a sensible granularity pairing: short ranges use hourly
// buckets, long ranges use weekly buckets — matches the dashboard convention.
function granularityForRangeDays(days: number): ReportTimeGranularity {
  if (days <= 1) return 'hour'
  if (days >= 29) return 'week'
  return 'day'
}

function detectQuickRangeDays(state: ReportFilterState): number | null {
  const days = Math.round(
    (state.endTimestamp - state.startTimestamp) / 86_400
  )
  return TIME_RANGE_PRESETS.some((preset) => preset.days === days)
    ? days
    : null
}

const SectionDivider = ({ label }: { label: string }) => (
  <div className='relative'>
    <div className='absolute inset-0 flex items-center'>
      <span className='w-full border-t' />
    </div>
    <div className='relative flex justify-center text-xs uppercase'>
      <span className='bg-background text-muted-foreground px-2'>
        {label}
      </span>
    </div>
  </div>
)

export function ReportFilterDialog({
  current,
  onApply,
  onReset,
}: ReportFilterDialogProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<ReportFilterState>(current)
  const [selectedRange, setSelectedRange] = useState<number | null>(() =>
    detectQuickRangeDays(current)
  )

  // Sync draft from the applied filters each time the dialog opens so a
  // previously applied manual range is preserved.
  useEffect(() => {
    if (open) {
      setDraft(current)
      setSelectedRange(detectQuickRangeDays(current))
    }
  }, [open, current])

  const handleQuickRange = (days: number) => {
    const { start, end } = getRollingDateRange(days)
    setDraft({
      startTimestamp: Math.floor(start.getTime() / 1000),
      endTimestamp: Math.floor(end.getTime() / 1000),
      granularity: granularityForRangeDays(days),
    })
    setSelectedRange(days)
  }

  const handleStartChange = (date?: Date) => {
    if (!date) return
    setDraft((prev) => ({
      ...prev,
      startTimestamp: Math.floor(date.getTime() / 1000),
    }))
    setSelectedRange(null)
  }

  const handleEndChange = (date?: Date) => {
    if (!date) return
    setDraft((prev) => ({
      ...prev,
      endTimestamp: Math.floor(date.getTime() / 1000),
    }))
    setSelectedRange(null)
  }

  const handleGranularity = (value: TimeGranularity) => {
    setDraft((prev) => ({ ...prev, granularity: value }))
  }

  const presets = useMemo(
    () =>
      TIME_RANGE_PRESETS.map((p) => ({
        ...p,
        label: t(p.label),
      })),
    [t]
  )

  return (
    <Dialog
      open={open}
      onOpenChange={setOpen}
      trigger={
        <Button variant='outline' size='sm'>
          <Filter className='mr-2 h-4 w-4' />
          {t('Filter')}
        </Button>
      }
      title={t('Reports Filters')}
      description={t(
        'Filter the reports view by time range and granularity.'
      )}
      contentClassName='max-sm:h-dvh max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-lg'
      contentHeight='min(48vh, 460px)'
      footerClassName='grid grid-cols-2 gap-2 sm:flex'
      footer={
        <>
          <Button
            onClick={() => {
              onReset()
              setOpen(false)
            }}
            variant='outline'
            type='button'
          >
            <RotateCcw className='mr-2 h-4 w-4' />
            {t('Reset')}
          </Button>
          <Button
            onClick={() => {
              onApply(draft)
              setOpen(false)
            }}
            type='submit'
          >
            <Search className='mr-2 h-4 w-4' />
            {t('Apply Filters')}
          </Button>
        </>
      }
    >
      <ScrollArea className='h-full pr-3 sm:pr-4'>
        <div className='grid gap-2.5 py-2'>
          {/* Quick time range */}
          <div className='grid gap-2'>
            <Label className='flex items-center gap-2'>
              <Calendar className='h-4 w-4' />
              {t('Quick Range')}
            </Label>
            <div className='grid grid-cols-2 gap-2 sm:flex'>
              {presets.map((range) => (
                <Button
                  key={range.days}
                  type='button'
                  size='sm'
                  variant={
                    selectedRange === range.days ? 'default' : 'outline'
                  }
                  onClick={() => handleQuickRange(range.days)}
                  className={cn(
                    'flex-1',
                    selectedRange === range.days &&
                      'ring-ring ring-2 ring-offset-2'
                  )}
                >
                  {range.label}
                </Button>
              ))}
            </div>
          </div>

          <SectionDivider label={t('Custom Time Range')} />

          {/* Custom time range */}
          <div className='grid gap-2.5'>
            <div className='grid gap-2'>
              <Label htmlFor='start_timestamp'>{t('Start Time')}</Label>
              <DateTimePicker
                value={new Date(draft.startTimestamp * 1000)}
                onChange={handleStartChange}
                placeholder={t('Select start time')}
              />
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='end_timestamp'>{t('End Time')}</Label>
              <DateTimePicker
                value={new Date(draft.endTimestamp * 1000)}
                onChange={handleEndChange}
                placeholder={t('Select end time')}
              />
            </div>
          </div>

          <SectionDivider label={t('Chart Settings')} />

          <div className='grid gap-2'>
            <Label htmlFor='time_granularity'>{t('Time Granularity')}</Label>
            <Select
              items={GRANULARITY_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              }))}
              value={draft.granularity}
              onValueChange={(value) =>
                handleGranularity(value as TimeGranularity)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t('Select time granularity')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {GRANULARITY_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {t(option.labelKey)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        </div>
      </ScrollArea>
    </Dialog>
  )
}
