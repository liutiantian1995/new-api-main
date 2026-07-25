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
import { GroupCombobox, type GroupOption } from '@/components/group-combobox'
import { getAdminPlans, type PlanRecord } from '@/features/subscriptions/api'
import { getAdminGroupDetails } from '@/lib/api'
import { batchCreateUsers } from '../api'
import { useUsers } from './users-provider'

const batchFormSchema = z
  .object({
    prefix: z
      .string()
      .min(1, 'Prefix is required')
      .max(10, 'Prefix must be at most 10 characters'),
    date_suffix: z.string().max(8).optional(),
    count: z
      .number()
      .min(1, 'Count must be at least 1')
      .max(200, 'Count must be at most 200'),
    group: z.string().min(1, 'Please select a group'),
    role: z.number().default(1),
    plan_id: z.number().optional(),
    activation_strategy: z.enum(['immediate', 'on_use']).default('immediate'),
    create_token: z.boolean().default(false),
    token_group: z.string().optional(),
  })
  .superRefine((data, ctx) => {
    // API token group is required when create_token is enabled. The backend
    // enforces the same invariant, so we surface it in the form to avoid a
    // round-trip rejection.
    if (data.create_token && !data.token_group) {
      ctx.addIssue({
        code: 'custom',
        path: ['token_group'],
        message: 'Please select a token group',
      })
    }
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
  token_group: '',
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

  // Fetch all system groups (admin-only). The 'auto' virtual group is
  // filtered out because user.Group semantics do not support 'auto'.
  const { data: groupDetailsData } = useQuery({
    queryKey: ['admin-group-details'],
    queryFn: getAdminGroupDetails,
    staleTime: 5 * 60 * 1000,
    enabled: open,
  })

  const plans: PlanRecord[] = plansData?.data || []
  // User-group dropdown filters out the 'auto' virtual group because
  // user.Group semantics do not support 'auto'.
  const groupOptions: GroupOption[] = (groupDetailsData?.data || [])
    .filter((g) => g.name !== 'auto')
    .map((g) => ({
      value: g.name,
      label: g.name,
      desc: g.desc || g.name,
      ratio: g.ratio,
    }))
  // Token-group dropdown KEEPS 'auto' because tokens may opt into the
  // virtual auto group for cross-group retry behavior.
  const tokenGroupOptions: GroupOption[] = (groupDetailsData?.data || []).map(
    (g) => ({
      value: g.name,
      label: g.name,
      desc: g.desc || g.name,
      ratio: g.ratio,
    })
  )

  const form = useForm<BatchFormValues>({
    resolver: zodResolver(batchFormSchema),
    defaultValues: DEFAULT_VALUES,
  })

  useEffect(() => {
    if (open) {
      form.reset(DEFAULT_VALUES)
    }
  }, [open, form])

  // Watch create_token to conditionally render the API token group field.
  // form.watch returns the current value reactively so the UI re-renders
  // when the switch toggles.
  const createToken = form.watch('create_token')

  const onSubmit = async (data: BatchFormValues) => {
    setIsSubmitting(true)
    try {
      const payload = {
        ...data,
        plan_id: data.plan_id || 0,
        date_suffix: data.date_suffix || '',
        group: data.group || '',
        token_group: data.create_token ? data.token_group || '' : '',
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
                      <GroupCombobox
                        options={groupOptions}
                        value={field.value}
                        onValueChange={field.onChange}
                        placeholder={t('Select a group')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Select the group assigned to each created user'
                      )}
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
                        onCheckedChange={(v) => {
                          field.onChange(v)
                          // Clear token_group when the switch turns off so
                          // validation does not fire on a hidden field.
                          if (!v) {
                            form.setValue('token_group', '')
                          }
                        }}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              {createToken && (
                <FormField
                  control={form.control}
                  name='token_group'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('API Token Group')}</FormLabel>
                      <FormControl>
                        <GroupCombobox
                          options={tokenGroupOptions}
                          value={field.value || ''}
                          onValueChange={field.onChange}
                          placeholder={t('Select a token group')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Select the group assigned to each created token'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
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
