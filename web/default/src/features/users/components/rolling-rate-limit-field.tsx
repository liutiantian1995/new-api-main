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
import { ChevronDown, ChevronUp, Plus, Trash2 } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  DURATION_PRESETS,
  formatDuration,
  type RollingRateLimitTier,
} from '@/features/system-settings/request-limits/rolling-rate-limit-types'

type Props = {
  value: string
  onChange: (v: string) => void
  groupName: string
}

function parseTiers(value: string): RollingRateLimitTier[] {
  if (!value || value.trim() === '') return []
  try {
    const parsed = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (t): t is RollingRateLimitTier =>
        t &&
        typeof t === 'object' &&
        typeof t.duration === 'number' &&
        typeof t.limit === 'number'
    )
  } catch {
    return []
  }
}

export function RollingRateLimitField({ value, onChange, groupName }: Props) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(() => parseTiers(value).length > 0)
  // Stable per-tier React keys; parallel to `tiers` and never reordered.
  // Decouples key identity from mutable duration/limit so input focus is
  // preserved when the user edits a value in place.
  const [tierIds, setTierIds] = useState<number[]>(() => {
    const n = parseTiers(value).length
    return Array.from({ length: n }, (_, i) => i)
  })
  let nextIdRef = useRef(0)
  if (tierIds.length > 0) {
    nextIdRef.current = Math.max(nextIdRef.current, ...tierIds) + 1
  }

  const tiers = parseTiers(value)

  const updateTiers = (next: RollingRateLimitTier[]) => {
    onChange(next.length === 0 ? '' : JSON.stringify(next))
  }
  const syncTierIds = (nextTiers: RollingRateLimitTier[], prevTiers: RollingRateLimitTier[]) => {
    if (nextTiers.length > prevTiers.length) {
      // Added: append fresh IDs for the new tail
      const added = nextTiers.length - prevTiers.length
      const start = nextIdRef.current
      nextIdRef.current += added
      setTierIds((prev) => [...prev, ...Array.from({ length: added }, (_, i) => start + i)])
    } else if (nextTiers.length < prevTiers.length) {
      // Removed: trim to the shorter length
      setTierIds((prev) => prev.slice(0, nextTiers.length))
    }
  }

  const addTier = (duration: number) => {
    const next = [...tiers, { duration, limit: 1000 }]
    syncTierIds(next, tiers)
    updateTiers(next)
  }

  const removeTier = (index: number) => {
    const next = tiers.filter((_, i) => i !== index)
    syncTierIds(next, tiers)
    updateTiers(next)
  }

  const updateTier = (index: number, field: 'duration' | 'limit', v: number) => {
    const next = [...tiers]
    next[index] = { ...next[index], [field]: v }
    // No structural change; tier IDs unchanged
    updateTiers(next)
  }

  return (
    <FormItem>
      <div className='flex items-center justify-between'>
        <FormLabel>{t('Rolling quota')}</FormLabel>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? (
            <ChevronUp className='h-4 w-4' />
          ) : (
            <ChevronDown className='h-4 w-4' />
          )}
          {expanded ? t('Collapse') : t('Custom quota')}
        </Button>
      </div>
      {!expanded && (
        <FormDescription>
          {t('Use group configuration')}: {groupName || 'default'}
        </FormDescription>
      )}
      {expanded && (
        <div className='space-y-2'>
          {tiers.length === 0 && (
            <p className='text-muted-foreground text-sm'>
              {t('No custom tiers — falling back to group default.')}
            </p>
          )}
          {tiers.map((tier, index) => (
            <div key={tierIds[index] ?? index} className='grid grid-cols-12 items-end gap-2'>
              <div className='col-span-5'>
                <FormLabel className='text-xs'>
                  {t('Duration (sec)')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={60}
                    value={tier.duration}
                    onChange={(e) => {
                      const v = Number.parseInt(e.target.value)
                      updateTier(index, 'duration', Number.isNaN(v) ? 0 : v)
                    }}
                  />
                </FormControl>
              </div>
              <div className='col-span-5'>
                <FormLabel className='text-xs'>{t('Limit')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    value={tier.limit}
                    onChange={(e) => {
                      const v = Number.parseInt(e.target.value)
                      updateTier(index, 'limit', Number.isNaN(v) ? 1 : v)
                    }}
                  />
                </FormControl>
              </div>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='col-span-2'
                onClick={() => removeTier(index)}
              >
                <Trash2 className='h-4 w-4' />
              </Button>
              <p className='text-muted-foreground col-span-12 text-xs'>
                {formatDuration(tier.duration)}: {tier.limit} {t('requests')}
              </p>
            </div>
          ))}
          <div className='flex flex-wrap gap-1'>
            {DURATION_PRESETS.map((p) => (
              <Button
                key={p.seconds}
                type='button'
                variant='outline'
                size='sm'
                onClick={() => addTier(p.seconds)}
              >
                <Plus className='mr-1 h-3 w-3' />
                {p.label}
              </Button>
            ))}
          </div>
          <FormDescription>
            {t(
              'Custom rolling quota overrides the group default for this user. Empty = use group default.'
            )}
          </FormDescription>
        </div>
      )}
      <FormMessage />
    </FormItem>
  )
}
