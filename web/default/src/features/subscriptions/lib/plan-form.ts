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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import type { SubscriptionPlan, PlanPayload } from '../types'

// HH:mm string → minutes-of-day (0-1439). Returns null for empty/invalid input.
function hhmmToMinutes(hhmm: string | null | undefined): number | null {
  if (!hhmm) return null
  const m = /^([01]?\d|2[0-3]):([0-5]\d)$/.exec(hhmm.trim())
  if (!m) return null
  return Number(m[1]) * 60 + Number(m[2])
}

// Minutes-of-day (0-1439) → "HH:mm". Returns null for null/undefined.
function minutesToHhmm(minutes: number | null | undefined): string | null {
  if (minutes == null || Number.isNaN(minutes)) return null
  const clamped = Math.max(0, Math.min(1439, Math.trunc(minutes)))
  if (clamped === 0) return null // 0/0 means all-day → empty form fields
  const h = Math.floor(clamped / 60)
  const mm = clamped % 60
  return `${String(h).padStart(2, '0')}:${String(mm).padStart(2, '0')}`
}

export function getPlanFormSchema(t: TFunction) {
  return z.object({
    title: z.string().min(1, t('Please enter plan title')),
    subtitle: z.string().optional(),
    price_amount: z.coerce.number().min(0, t('Please enter amount')),
    duration_unit: z.enum(['year', 'month', 'day', 'hour', 'custom']),
    duration_value: z.coerce.number().min(1),
    custom_seconds: z.coerce.number().min(0).optional(),
    quota_reset_period: z.enum([
      'never',
      'daily',
      'weekly',
      'monthly',
      'custom',
    ]),
    quota_reset_custom_seconds: z.coerce.number().min(0).optional(),
    daily_active_start: z.string().nullable().optional(),
    daily_active_end: z.string().nullable().optional(),
    enabled: z.boolean(),
    sort_order: z.coerce.number(),
    allow_balance_pay: z.boolean(),
    allow_wallet_overflow: z.boolean(),
    max_purchase_per_user: z.coerce.number().min(0),
    total_amount: z.coerce.number().min(0),
    upgrade_group: z.string().optional(),
    downgrade_group: z.string().optional(),
    stripe_price_id: z.string().optional(),
    creem_product_id: z.string().optional(),
    waffo_pancake_product_id: z.string().optional(),
  }).superRefine((val, ctx) => {
    const start = val.daily_active_start ?? null
    const end = val.daily_active_end ?? null
    // Either both filled or both empty. Partial fill is invalid.
    if ((start == null) !== (end == null)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: t('Daily active start and end must both be set or both be empty'),
        path: ['daily_active_start'],
      })
    }
  })
}

export type PlanFormValues = z.infer<ReturnType<typeof getPlanFormSchema>>

export const PLAN_FORM_DEFAULTS: PlanFormValues = {
  title: '',
  subtitle: '',
  price_amount: 0,
  duration_unit: 'month',
  duration_value: 1,
  custom_seconds: 0,
  quota_reset_period: 'never',
  quota_reset_custom_seconds: 0,
  daily_active_start: null,
  daily_active_end: null,
  enabled: true,
  sort_order: 0,
  allow_balance_pay: true,
  allow_wallet_overflow: true,
  max_purchase_per_user: 0,
  total_amount: 0,
  upgrade_group: '',
  downgrade_group: '',
  stripe_price_id: '',
  creem_product_id: '',
  waffo_pancake_product_id: '',
}

export function planToFormValues(plan: SubscriptionPlan): PlanFormValues {
  const startMin = Number(plan.daily_active_start_minutes || 0)
  const endMin = Number(plan.daily_active_end_minutes || 0)
  return {
    title: plan.title || '',
    subtitle: plan.subtitle || '',
    price_amount: Number(plan.price_amount || 0),
    duration_unit: plan.duration_unit || 'month',
    duration_value: Number(plan.duration_value || 1),
    custom_seconds: Number(plan.custom_seconds || 0),
    quota_reset_period: plan.quota_reset_period || 'never',
    quota_reset_custom_seconds: Number(plan.quota_reset_custom_seconds || 0),
    // Only populate form fields when window is not all-day (0/0).
    daily_active_start: startMin === 0 && endMin === 0 ? null : minutesToHhmm(startMin),
    daily_active_end: startMin === 0 && endMin === 0 ? null : minutesToHhmm(endMin),
    enabled: plan.enabled !== false,
    sort_order: Number(plan.sort_order || 0),
    allow_balance_pay: plan.allow_balance_pay !== false,
    allow_wallet_overflow: plan.allow_wallet_overflow !== false,
    max_purchase_per_user: Number(plan.max_purchase_per_user || 0),
    total_amount: quotaUnitsToDollars(Number(plan.total_amount || 0)),
    upgrade_group: plan.upgrade_group || '',
    downgrade_group: plan.downgrade_group || '',
    stripe_price_id: plan.stripe_price_id || '',
    creem_product_id: plan.creem_product_id || '',
    waffo_pancake_product_id: plan.waffo_pancake_product_id || '',
  }
}

export function formValuesToPlanPayload(values: PlanFormValues): PlanPayload {
  const startMin = hhmmToMinutes(values.daily_active_start ?? null)
  const endMin = hhmmToMinutes(values.daily_active_end ?? null)
  return {
    plan: {
      ...values,
      price_amount: Number(values.price_amount || 0),
      currency: 'USD',
      duration_value: Number(values.duration_value || 0),
      custom_seconds: Number(values.custom_seconds || 0),
      quota_reset_period: values.quota_reset_period || 'never',
      quota_reset_custom_seconds:
        values.quota_reset_period === 'custom'
          ? Number(values.quota_reset_custom_seconds || 0)
          : 0,
      // Both null/undefined → 0/0 (all-day). Otherwise emit minutes-of-day.
      daily_active_start_minutes: startMin == null ? 0 : startMin,
      daily_active_end_minutes: endMin == null ? 0 : endMin,
      sort_order: Number(values.sort_order || 0),
      max_purchase_per_user: Number(values.max_purchase_per_user || 0),
      total_amount: parseQuotaFromDollars(Number(values.total_amount || 0)),
      upgrade_group: values.upgrade_group || '',
      downgrade_group: values.downgrade_group || '',
    },
  }
}
