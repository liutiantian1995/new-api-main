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
export interface RollingRateLimitTier {
  duration: number
  limit: number
}

export interface RollingRateLimitGroupConfig {
  [groupName: string]: RollingRateLimitTier[]
}

// Mirrors backend common.FormatRollingDuration (common/format-duration.go)
export function formatDuration(sec: number): string {
  if (sec <= 0) return ''
  const hour = 3600
  const day = 86400
  const week = 604800
  if (sec === week) return '1 周'
  if (sec === day) return '1 天'
  if (sec % day === 0) return `${sec / day} 天`
  if (sec % hour === 0) return `${sec / hour} 小时`
  const hours = Math.ceil(sec / hour)
  if (hours <= 0) return '1 小时'
  return `${hours} 小时`
}

export const DURATION_PRESETS = [
  { label: '5 小时', seconds: 18000 },
  { label: '1 天', seconds: 86400 },
  { label: '1 周', seconds: 604800 },
  { label: '30 天', seconds: 2592000 },
] as const
