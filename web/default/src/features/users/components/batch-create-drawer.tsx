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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'
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
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { getAdminPlans, type PlanRecord } from '@/features/subscriptions/api'
import { batchCreateUsers } from '../api'
import { useUsers } from './users-provider'

const batchFormSchema = z.object({
  prefix: z
    .string()
    .min(1, 'Prefix is required')
    .max(10, 'Prefix must be at most 10 characters'),
  date_suffix: z.string().max(8).optional(),
  count: z
    .number()
    .min(1, 'Count must be at least 1')
    .max(200, 'Count must be at most 200'),
  group: z.string().optional(),
  role: z.number().default(1),
  plan_id: z.number().optional(),
  activation_strategy: z.enum(['immediate', 'on_use']).default('immediate'),
  create_token: z.boolean().default(false),
})

type BatchFormValues = z.infer<typeof batchFormSchema>

// 默认日期后缀：当天 MMDD（如 0705），基于浏览器本地时区
const getDefaultDateSuffix = (): string => {
  const now = new Date()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${month}${day}`
}

const DEFAULT_VALUES: BatchFormValues = {
  prefix: '',
  date_suffix: getDefaultDateSuffix(),
  count: 10,
  group: '',
  role: 1,
  plan_id: 0,
  activation_strategy: 'immediate',
  create_token: false,
}

type BatchCreateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function BatchCreateDrawer({
  open,
  onOpenChange,
}: BatchCreateDrawerProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useUsers()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const { data: plansData } = useQuery({
    queryKey: ['admin-plans'],
    queryFn: getAdminPlans,
    staleTime: 5 * 60 * 1000,
    enabled: open,
  })

  const plans: PlanRecord[] = plansData?.data || []

  const form = useForm<BatchFormValues>({
    resolver: zodResolver(batchFormSchema),
    defaultValues: DEFAULT_VALUES,
  })

  useEffect(() => {
    if (open) {
      form.reset(DEFAULT_VALUES)
    }
  }, [open, form])

  const onSubmit = async (data: BatchFormValues) => {
    setIsSubmitting(true)
    try {
      const payload = {
        ...data,
        plan_id: data.plan_id || 0,
        date_suffix: data.date_suffix || '',
        group: data.group || '',
      }
      const result = await batchCreateUsers(payload)
      if (result.success) {
        toast.success(
          result.data?.message ||
            t('Batch creation successful, initial password is username@123')
        )
        onOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(result.message || t('Batch creation failed'))
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      toast.error(message || t('Batch creation failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle className='flex items-center gap-2'>
            <Users className='h-5 w-5' />
            {t('Batch Create Users')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Create multiple users at once with generated usernames and passwords'
            )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='prefix'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Username Prefix')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. user')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Usernames will be generated as {prefix}{date}{random}, e.g. user0601ab3x7y'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='date_suffix'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Date Suffix')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. 0601 (MMDD)')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Defaults to today (MMDD), leave to use today')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='count'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Count')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={200}
                        {...field}
                        onChange={(e) =>
                          field.onChange(Number(e.target.value))
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Maximum 200 users per batch')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Group')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('default')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Leave empty to use default group')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <FormField
                control={form.control}
                name='plan_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Subscription Plan')}</FormLabel>
                    <Select
                      value={field.value ? String(field.value) : ''}
                      onValueChange={(v) =>
                        field.onChange(v ? Number(v) : 0)
                      }
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t('No subscription plan')}
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectGroup>
                          {plans.map((p) => (
                            <SelectItem
                              key={p.plan.id}
                              value={String(p.plan.id)}
                            >
                              {p.plan.title} ($
                              {Number(p.plan.price_amount || 0).toFixed(2)})
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'Optional: bind a subscription plan to each created user'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='activation_strategy'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Activation Strategy')}</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={(v) =>
                        v !== null && field.onChange(v)
                      }
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='immediate'>
                          {t('Immediate')}
                        </SelectItem>
                        <SelectItem value='on_use'>
                          {t('On Use')}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'On Use: subscription starts counting when user first logs in or uses a token'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='create_token'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                    <div className='space-y-0.5'>
                      <FormLabel>{t('Create API Token')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Create a default API token for each user with unlimited quota'
                        )}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SheetFooter className={sideDrawerFooterClassName()}>
              <SheetClose asChild>
                <Button
                  variant='outline'
                  type='button'
                  disabled={isSubmitting}
                >
                  {t('Cancel')}
                </Button>
              </SheetClose>
              <Button type='submit' disabled={isSubmitting}>
                {isSubmitting ? t('Creating...') : t('Batch Create')}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
