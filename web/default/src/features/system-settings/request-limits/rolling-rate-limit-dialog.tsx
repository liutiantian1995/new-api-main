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
import { zodResolver } from '@hookform/resolvers/zod'
import { Trash2 } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'

import {
  DURATION_PRESETS,
  type RollingRateLimitTier,
} from './rolling-rate-limit-types'

const tierSchema = z.object({
  duration: z.number().min(60, 'Must be ≥ 60').max(2147483647),
  limit: z.number().min(1, 'Must be ≥ 1').max(2147483647),
})

const dialogSchema = z
  .object({
    groupName: z.string().min(1, 'Group name is required'),
    tiers: z
      .array(tierSchema)
      .min(1, 'At least 1 tier required')
      .max(5, 'Max 5 tiers')
      .refine((tiers) => new Set(tiers.map((t) => t.duration)).size === tiers.length, {
        message: 'Duplicate durations not allowed',
      }),
  })

type DialogFormValues = z.infer<typeof dialogSchema>

const FORM_ID = 'rolling-rate-limit-dialog-form'

export type RollingRateLimitDialogData = {
  groupName: string
  tiers: RollingRateLimitTier[]
}

type Props = {
  open: boolean
  onOpenChange: (v: boolean) => void
  onSave: (data: RollingRateLimitDialogData) => void
  editData?: RollingRateLimitDialogData | null
}

export function RollingRateLimitDialog({
  open,
  onOpenChange,
  onSave,
  editData,
}: Props) {
  const { t } = useTranslation()
  const isEdit = !!editData
  // Stable per-row keys so React reconciles correctly when tiers are
  // added/removed. Kept in a ref so they survive re-renders.
  const tierKeyRef = useRef<string[]>(['tier-seed-0'])
  const nextKeyRef = useRef(1)
  const ensureTierKeys = (count: number) => {
    const keys = tierKeyRef.current
    if (keys.length === count) return
    if (keys.length < count) {
      while (keys.length < count) {
        keys.push(`tier-seed-${nextKeyRef.current++}`)
      }
    } else {
      keys.length = count
    }
  }

  const form = useForm<DialogFormValues>({
    resolver: zodResolver(dialogSchema),
    defaultValues: { groupName: '', tiers: [{ duration: 18000, limit: 500 }] },
  })

  useEffect(() => {
    if (editData) {
      form.reset({ groupName: editData.groupName, tiers: editData.tiers })
    } else {
      form.reset({ groupName: '', tiers: [{ duration: 18000, limit: 500 }] })
    }
    // Reset keys to match the incoming tier count on every open/edit cycle.
    tierKeyRef.current = []
    nextKeyRef.current = 1
    const count = (editData?.tiers?.length ?? 1) || 1
    ensureTierKeys(count)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editData, open])

  const tiers = form.watch('tiers')
  ensureTierKeys(tiers?.length ?? 0)

  const handleSubmit = (values: DialogFormValues) => {
    onSave({ groupName: values.groupName, tiers: values.tiers })
    onOpenChange(false)
  }

  const addTier = (seconds: number) => {
    form.setValue('tiers', [...form.getValues('tiers'), { duration: seconds, limit: 1000 }])
    tierKeyRef.current.push(`tier-seed-${nextKeyRef.current++}`)
  }

  const removeTier = (index: number) => {
    form.setValue('tiers', form.getValues('tiers').filter((_, i) => i !== index))
    tierKeyRef.current.splice(index, 1)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={isEdit ? t('Edit rolling quota') : t('Add rolling quota')}
      description={t(
        'Configure multi-tier rolling window request quotas for a user group.'
      )}
      contentClassName='sm:max-w-[600px]'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button type='submit' form={FORM_ID}>
            {isEdit ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form id={FORM_ID} onSubmit={form.handleSubmit(handleSubmit)} className='space-y-4'>
          <FormField
            control={form.control}
            name='groupName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Group Name')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('e.g., default, vip, premium')}
                    {...field}
                    disabled={isEdit}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='space-y-2'>
            <div className='flex items-center justify-between'>
              <FormLabel>{t('Rolling Window Tiers (max 5)')}</FormLabel>
              <div className='flex gap-1'>
                {DURATION_PRESETS.map((p) => (
                  <Button
                    key={p.seconds}
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => addTier(p.seconds)}
                  >
                    {p.label}
                  </Button>
                ))}
              </div>
            </div>
            {tiers?.map((_tier, index) => (
              <div key={tierKeyRef.current[index] ?? `tier-seed-${index}`} className='grid grid-cols-12 items-end gap-2'>
                <FormField
                  control={form.control}
                  name={`tiers.${index}.duration`}
                  render={({ field }) => (
                    <FormItem className='col-span-5'>
                      <FormLabel className='text-xs'>
                        {t('Duration (sec)')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={60}
                          {...field}
                          onChange={(e) => {
                            const v = Number.parseInt(e.target.value)
                            field.onChange(Number.isNaN(v) ? 0 : v)
                          }}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={`tiers.${index}.limit`}
                  render={({ field }) => (
                    <FormItem className='col-span-5'>
                      <FormLabel className='text-xs'>{t('Limit')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={1}
                          {...field}
                          onChange={(e) =>
                            field.onChange(Number.parseInt(e.target.value) || 1)
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='col-span-2'
                  onClick={() => removeTier(index)}
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
            ))}
            <FormDescription>
              {t(
                'First tier to be exceeded triggers 429. Durations must be unique.'
              )}
            </FormDescription>
          </div>
        </form>
      </Form>
    </Dialog>
  )
}
