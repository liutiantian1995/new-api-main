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
import { Download, Plus, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getRouteApi } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'
import { useUsers } from './users-provider'

// 复用 users-table 中相同的 route api，从 URL 读取当前 globalFilter 与 group 过滤
const route = getRouteApi('/_authenticated/users/')

export function UsersPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useUsers()
  // 从 URL search state 读取：globalFilter 对应 keyword，group 对应分组过滤
  // search schema 见 routes/_authenticated/users/index.tsx 的 usersSearchSchema
  const search = route.useSearch()
  const keyword = search.filter ?? ''
  const group = search.group ?? ''

  const handleCreate = () => {
    setCurrentRow(null)
    setOpen('create')
  }

  const handleBatchCreate = () => {
    setOpen('batch_create')
  }

  // 下载当前搜索条件下的用户凭据 TXT（用户/密码/key 三行一组）
  // 与列表过滤保持一致：复用 URL 中的 keyword 与 group；后端硬性上限 10000 条
  // 走 api axios 实例：拦截器自动注入 New-Api-User header，
  // 浏览器导航（window.location.href）不会携带该 header，会被鉴权中间件拒绝
  const handleExport = async () => {
    const params = new URLSearchParams()
    if (keyword) {
      params.set('keyword', keyword)
    }
    if (group) {
      params.set('group', group)
    }
    const qs = params.toString()
    const url = '/api/user/export' + (qs ? '?' + qs : '')
    try {
      const res = await api.get(url, { responseType: 'blob' })
      // 从 Content-Disposition 解析文件名，回退到 users.txt
      const cd = res.headers['content-disposition'] || ''
      const m = /filename="?([^";]+)"?/.exec(cd)
      const blob = new Blob([res.data], { type: 'text/plain;charset=utf-8;' })
      const downloadUrl = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = downloadUrl
      a.download = m ? m[1] : 'users.txt'
      a.click()
      URL.revokeObjectURL(downloadUrl)
    } catch {
      // 错误已在 api 拦截器中 toast 提示
    }
  }

  return (
    <div className='flex gap-2'>
      <Button size='sm' variant='outline' onClick={handleBatchCreate}>
        <Users className='h-4 w-4' />
        {t('Batch Create')}
      </Button>
      <Button size='sm' variant='outline' onClick={handleExport}>
        <Download className='h-4 w-4' />
        {t('Download Credentials')}
      </Button>
      <Button size='sm' onClick={handleCreate}>
        <Plus className='h-4 w-4' />
        {t('Add User')}
      </Button>
    </div>
  )
}
