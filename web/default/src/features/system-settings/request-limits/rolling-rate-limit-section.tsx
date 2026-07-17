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
import { Code2, Palette } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { RollingRateLimitVisualEditor } from './rolling-rate-limit-visual-editor'

const schema = z.object({
  UserRollingRateLimitEnabled: z.boolean(),
  UserRollingRateLimitGroup: z.string().optional(),
})

type FormValues = z.infer<typeof schema>

type Props = { defaultValues: FormValues }

export function RollingRateLimitSection({ defaultValues }: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [useVisual, setUseVisual] = useState(true)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    mode: 'onChange',
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: FormValues) => {
    for (const [key, value] of Object.entries(values)) {
      if (value !== defaultValues[key as keyof FormValues]) {
        await updateOption.mutateAsync({ key, value: value ?? '' })
      }
    }
  }

  return (
    <SettingsSection title={t('User Rolling Quota')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel={t('Save rolling quotas')}
          />

          <FormField
            control={form.control}
            name='UserRollingRateLimitEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable user rolling quota')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Complements the minute-rate limiter: this controls long-term request quotas (5h / 1d / 1w).'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='UserRollingRateLimitGroup'
            render={({ field }) => (
              <FormItem>
                <div className='flex items-center justify-between'>
                  <FormLabel>{t('Group-based rolling quotas')}</FormLabel>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => setUseVisual(!useVisual)}
                  >
                    {useVisual ? (
                      <>
                        <Code2 className='mr-2 h-4 w-4' />
                        {t('JSON Mode')}
                      </>
                    ) : (
                      <>
                        <Palette className='mr-2 h-4 w-4' />
                        {t('Visual Mode')}
                      </>
                    )}
                  </Button>
                </div>
                <FormControl>
                  {useVisual ? (
                    <RollingRateLimitVisualEditor
                      value={field.value || ''}
                      onChange={field.onChange}
                    />
                  ) : (
                    <Textarea
                      rows={8}
                      placeholder={`{\n  "default": [{"duration": 18000, "limit": 500}]\n}`}
                      className='font-mono text-sm'
                      {...field}
                    />
                  )}
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
