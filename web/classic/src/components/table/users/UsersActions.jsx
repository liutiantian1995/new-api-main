/*
Copyright (C) 2025 QuantumNous

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

import React from 'react';
import { Button } from '@douyinfe/semi-ui';
import { IconDownload } from '@douyinfe/semi-icons';
import { API } from '../../../helpers/api';

const UsersActions = ({
  setShowAddUser,
  setShowBatchCreate,
  searchKeyword,
  searchGroup,
  t,
}) => {
  // Add new user
  const handleAddUser = () => {
    setShowAddUser(true);
  };

  const handleBatchCreate = () => {
    setShowBatchCreate?.(true);
  };

  // 下载当前搜索结果下的用户凭据 CSV（username/password/api_key）
  // 复用列表的 keyword/group 过滤；后端硬性上限 10000 条
  // 走 axios 实例：API 已在 helpers/api.js 中静态注入 New-API-User header，
  // 浏览器导航（window.location.href）不会携带该 header，会被鉴权中间件拒绝
  const handleExport = async () => {
    const params = new URLSearchParams();
    if (searchKeyword) {
      params.set('keyword', searchKeyword);
    }
    if (searchGroup) {
      params.set('group', searchGroup);
    }
    const qs = params.toString();
    const url = '/api/user/export' + (qs ? '?' + qs : '');
    try {
      const res = await API.get(url, { responseType: 'blob' });
      // 从 Content-Disposition 解析文件名，回退到 users.csv
      const cd = res.headers['content-disposition'] || '';
      const m = /filename="?([^";]+)"?/.exec(cd);
      const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8;' });
      const downloadUrl = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = downloadUrl;
      a.download = m ? m[1] : 'users.csv';
      a.click();
      URL.revokeObjectURL(downloadUrl);
    } catch {
      // 错误已在 API 拦截器中 showError 提示
    }
  };

  return (
    <div className='flex gap-2 w-full md:w-auto order-2 md:order-1'>
      <Button
        className='w-full md:w-auto'
        onClick={handleBatchCreate}
        size='small'
        theme='light'
      >
        {t('批量创建用户')}
      </Button>
      <Button
        className='w-full md:w-auto'
        onClick={handleExport}
        size='small'
        theme='light'
        icon={<IconDownload />}
      >
        {t('下载凭据')}
      </Button>
      <Button className='w-full md:w-auto' onClick={handleAddUser} size='small'>
        {t('添加用户')}
      </Button>
    </div>
  );
};

export default UsersActions;
