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
import {
  Hash,
  Coins,
  Layers,
  Gauge,
  Zap,
  Flame,
  TrendingUp,
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { safeDivide } from '@/features/dashboard/lib'

interface StatCardConfig {
  key: string
  title: string
  description: string
  icon: LucideIcon
  getValue: (stat: Record<string, number>, days?: number) => number
}

export function useModelStatCardsConfig(): StatCardConfig[] {
  const { t } = useTranslation()

  return [
    {
      key: 'count',
      title: t('Total Count'),
      description: t('Statistical count'),
      icon: Hash,
      // 用 request_count（时间窗口内真实总请求数），不要用 rpm——rpm 只统计最近 60 秒，
      // 当短时间内无新请求时总数会显示 0，与 "Total Count" 的语义不符。
      getValue: (stat) => stat?.request_count ?? stat?.rpm ?? 0,
    },
    {
      key: 'quota',
      title: t('Total Quota'),
      description: t('Statistical quota'),
      icon: Coins,
      getValue: (stat) => stat?.quota ?? 0,
    },
    {
      key: 'inputTokens',
      title: t('Input Tokens'),
      description: t('Prompt tokens'),
      icon: ArrowDownToLine,
      getValue: (stat) => stat?.prompt_tokens ?? 0,
    },
    {
      key: 'cachedTokens',
      title: t('Cache Hit'),
      description: t('Cached prompt tokens'),
      icon: Layers,
      getValue: (stat) => stat?.cached_tokens ?? 0,
    },
    {
      key: 'outputTokens',
      title: t('Output Tokens'),
      description: t('Completion tokens'),
      icon: ArrowUpFromLine,
      getValue: (stat) => stat?.completion_tokens ?? 0,
    },
    {
      key: 'tokens',
      title: t('Total Tokens'),
      description: t('Statistical tokens'),
      icon: Layers,
      // 用 total_tokens（窗口内真实总 tokens），不要用 tpm--tpm 只统计最近 60 秒，
      // 当短时间内无新请求时总数会显示 0，与 "Total Tokens" 的语义不符。
      getValue: (stat) => stat?.total_tokens ?? stat?.tpm ?? 0,
    },
    {
      key: 'avgRpm',
      title: t('Average RPM'),
      description: t('Requests per minute'),
      icon: Gauge,
      // 窗口内平均每分钟请求数 = 总请求数 / 时间窗口分钟数
      // 用 request_count 而不是 rpm（rpm 是 60 秒瞬时值，除以 minutes 后接近 0）
      getValue: (stat, timeRangeMinutes = 1) =>
        safeDivide(stat?.request_count ?? stat?.rpm ?? 0, timeRangeMinutes),
    },
    {
      key: 'avgTpm',
      title: t('Average TPM'),
      description: t('Tokens per minute'),
      icon: Zap,
      // 窗口内平均每分钟 tokens = 总 tokens / 时间窗口分钟数
      // 用 total_tokens 而不是 tpm（tpm 是 60 秒瞬时值，除以 minutes 后接近 0）
      getValue: (stat, timeRangeMinutes = 1) =>
        safeDivide(stat?.total_tokens ?? stat?.tpm ?? 0, timeRangeMinutes),
    },
  ]
}

export function useSummaryCardsConfig(totals: {
  todayUsageDisplay: string
  usedDisplay: string
  requestCountDisplay: string
  currencyLabel: string
  currencyEnabled: boolean
}) {
  const { t } = useTranslation()

  return [
    {
      key: 'todayUsage',
      title: t('Last 24h usage'),
      value: totals.todayUsageDisplay,
      description: totals.currencyEnabled
        ? `${t('Consumed in the last 24 hours')} (${totals.currencyLabel})`
        : t('Consumed in the last 24 hours'),
      icon: Flame,
    },
    {
      key: 'usage',
      title: t('Historical Usage'),
      value: totals.usedDisplay,
      description: totals.currencyEnabled
        ? `${t('Total consumed')} (${totals.currencyLabel})`
        : t('Total consumed quota'),
      icon: TrendingUp,
    },
    {
      key: 'requests',
      title: t('Request Count'),
      value: totals.requestCountDisplay,
      description: t('Total requests made'),
      icon: Activity,
    },
  ]
}
