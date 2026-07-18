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
import { useFieldArray, useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2, Info } from 'lucide-react'

import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import type { ChannelFormValues } from '../../lib/channel-form'

// TokenRoutingSection — token 感知路由配置面板（default 主题）。
//
// 接入 react-hook-form：max_tokens 作为顶层 number 字段；token_tiers 作为
// useFieldArray 管理的数组字段。提交时随 form values 直接发送给后端。
export function TokenRoutingSection() {
  const { t } = useTranslation()
  const form = useFormContext<ChannelFormValues>()
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'token_tiers',
  })

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex items-start gap-2 rounded-md border border-blue-500/20 bg-blue-500/5 p-3 text-xs'>
        <Info className='mt-0.5 h-3.5 w-3.5 shrink-0 text-blue-500' />
        <span className='text-muted-foreground'>
          {t(
            'Route requests by estimated token count. Requests with estTokens exceeding max_tokens skip this channel; smaller requests receive a priority boost from configured tiers. No configuration = original routing behavior.'
          )}
        </span>
      </div>

      <FormField
        control={form.control}
        name='max_tokens'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Max Tokens')}</FormLabel>
            <FormControl>
              <Input
                type='number'
                min={0}
                placeholder='0'
                value={field.value ?? 0}
                onChange={(e) => field.onChange(Number(e.target.value))}
              />
            </FormControl>
            <FormDescription>
              {t(
                'estTokens exceeding max_tokens will skip this channel. 0 = unlimited.'
              )}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className='flex flex-col gap-2'>
        <Label className='text-[13px] font-medium'>
          {t('Token Tiers (priority_boost by token threshold)')}
        </Label>

        {fields.length === 0 && (
          <div className='rounded-md border border-dashed p-3 text-xs text-muted-foreground'>
            {t(
              'No tiers configured. Click "Add Tier" to provide a priority boost for smaller requests.'
            )}
          </div>
        )}

        {fields.map((field, idx) => (
          <div
            key={field.id}
            className='grid grid-cols-[1fr_1fr_auto] items-center gap-2'
          >
            <FormField
              control={form.control}
              name={`token_tiers.${idx}.max_tokens` as const}
              render={({ field: tierField }) => (
                <FormItem>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      placeholder='≤ tokens'
                      value={tierField.value ?? ''}
                      onChange={(e) =>
                        tierField.onChange(Number(e.target.value))
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name={`token_tiers.${idx}.priority_boost` as const}
              render={({ field: tierField }) => (
                <FormItem>
                  <FormControl>
                    <Input
                      type='number'
                      min={-100}
                      max={100}
                      placeholder='priority Δ'
                      value={tierField.value ?? ''}
                      onChange={(e) =>
                        tierField.onChange(Number(e.target.value))
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
              size='icon'
              onClick={() => remove(idx)}
              aria-label={t('Remove tier')}
            >
              <Trash2 className='h-4 w-4' />
            </Button>
          </div>
        ))}

        <Button
          type='button'
          variant='outline'
          size='sm'
          className='w-fit'
          onClick={() => append({ max_tokens: 50000, priority_boost: 0 })}
          disabled={fields.length >= 10}
        >
          <Plus className='mr-1 h-4 w-4' />
          {t('Add Tier')}
        </Button>
        {fields.length >= 10 && (
          <span className='text-xs text-muted-foreground'>
            {t('Up to 10 tiers')}
          </span>
        )}
      </div>
    </div>
  )
}
