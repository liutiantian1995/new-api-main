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
import { Download, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getRouteApi } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'
import { useRedemptions } from './redemptions-provider'

// 复用 redemptions-table 中相同的 route api，从 URL 读取当前 globalFilter
const route = getRouteApi('/_authenticated/redemption-codes/')

export function RedemptionsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useRedemptions()
  // 从 URL search state 读取：filter 对应 keyword
  const search = route.useSearch()
  const keyword = search.filter ?? ''

  // 下载当前搜索条件下的兑换码 TXT（每行「名称: 代码」）
  // 与列表过滤保持一致：复用 URL 中的 keyword；后端硬性上限 10000 条
  // 走 api axios 实例：拦截器自动注入 New-Api-User header，
  // 浏览器导航（window.location.href）不会携带该 header，会被鉴权中间件拒绝
  const handleExport = async () => {
    const params = new URLSearchParams()
    if (keyword) {
      params.set('keyword', keyword)
    }
    const qs = params.toString()
    const url = '/api/redemption/export' + (qs ? '?' + qs : '')
    try {
      const res = await api.get(url, { responseType: 'blob' })
      // 从 Content-Disposition 解析文件名，回退到 redemptions.txt
      const cd = res.headers['content-disposition'] || ''
      const m = /filename="?([^";]+)"?/.exec(cd)
      const blob = new Blob([res.data], { type: 'text/plain;charset=utf-8;' })
      const downloadUrl = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = downloadUrl
      a.download = m ? m[1] : 'redemptions.txt'
      a.click()
      URL.revokeObjectURL(downloadUrl)
    } catch {
      // 错误已在 api 拦截器中 toast 提示
    }
  }

  return (
    <div className='flex gap-2'>
      <Button size='sm' variant='outline' onClick={handleExport}>
        <Download className='h-4 w-4' />
        {t('Download Codes')}
      </Button>
      <Button size='sm' onClick={() => setOpen('create')}>
        <Plus className='h-4 w-4' />
        {t('Create Code')}
      </Button>
    </div>
  )
}
