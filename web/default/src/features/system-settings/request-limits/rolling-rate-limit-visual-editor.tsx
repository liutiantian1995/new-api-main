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
import { Plus } from 'lucide-react'
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  StaticDataTable,
  StaticRowActions,
} from '@/components/data-table'
import { Button } from '@/components/ui/button'

import { safeJsonParseWithValidation } from '../utils/json-parser'
import {
  formatDuration,
  type RollingRateLimitGroupConfig,
  type RollingRateLimitTier,
} from './rolling-rate-limit-types'
import {
  RollingRateLimitDialog,
  type RollingRateLimitDialogData,
} from './rolling-rate-limit-dialog'

function isRollingRateLimitGroupConfig(
  data: unknown
): data is RollingRateLimitGroupConfig {
  if (typeof data !== 'object' || data === null || Array.isArray(data)) {
    return false
  }
  for (const value of Object.values(data as Record<string, unknown>)) {
    if (!Array.isArray(value)) return false
    for (const tier of value) {
      if (
        typeof tier !== 'object' ||
        tier === null ||
        Array.isArray(tier) ||
        typeof (tier as Record<string, unknown>).duration !== 'number' ||
        typeof (tier as Record<string, unknown>).limit !== 'number'
      ) {
        return false
      }
    }
  }
  return true
}

type Props = {
  value: string
  onChange: (v: string) => void
}

export function RollingRateLimitVisualEditor({ value, onChange }: Props) {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editData, setEditData] = useState<RollingRateLimitDialogData | null>(null)

  const config = useMemo<RollingRateLimitGroupConfig>(() => {
    if (!value || value.trim() === '') return {}
    return safeJsonParseWithValidation<RollingRateLimitGroupConfig>(value, {
      fallback: {},
      validator: isRollingRateLimitGroupConfig,
      validatorMessage: 'Rolling rate limit group must be a JSON object',
      context: 'rolling rate limit group',
    })
  }, [value])

  const entries = useMemo(() => Object.entries(config), [config])

  const handleSave = (data: RollingRateLimitDialogData) => {
    const next = { ...config }
    if (editData && editData.groupName !== data.groupName) {
      delete next[editData.groupName]
    }
    next[data.groupName] = data.tiers
    onChange(JSON.stringify(next, null, 2))
  }

  const handleDelete = (groupName: string) => {
    const next = { ...config }
    delete next[groupName]
    onChange(JSON.stringify(next, null, 2))
  }

  return (
    <div className='space-y-4'>
      <div className='flex justify-end'>
        <Button
          onClick={() => {
            setEditData(null)
            setDialogOpen(true)
          }}
        >
          <Plus className='mr-2 h-4 w-4' />
          {t('Add group')}
        </Button>
      </div>

      <StaticDataTable
        data={entries}
        getRowKey={([name]) => name}
        emptyContent={
          t(
            'No rolling window quotas configured. Click "Add group" to get started.'
          )
        }
        columns={[
          { id: 'group', header: t('Group Name'), cell: ([name]) => name },
          {
            id: 'tiers',
            header: t('Tiers'),
            cell: ([, tiers]) =>
              (tiers as RollingRateLimitTier[])
                .map(
                  (tier) => `${formatDuration(tier.duration)}: ${tier.limit}`
                )
                .join(', '),
          },
          {
            id: 'actions',
            header: t('Actions'),
            className: 'text-right',
            cell: ([name, tiers]) => (
              <StaticRowActions
                editLabel={t('Edit')}
                deleteLabel={t('Delete')}
                menuLabel={t('Open menu')}
                onEdit={() => {
                  setEditData({
                    groupName: name,
                    tiers: tiers as RollingRateLimitTier[],
                  })
                  setDialogOpen(true)
                }}
                onDelete={() => handleDelete(name)}
              />
            ),
          },
        ]}
      />

      <RollingRateLimitDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSave={handleSave}
        editData={editData}
      />
    </div>
  )
}
