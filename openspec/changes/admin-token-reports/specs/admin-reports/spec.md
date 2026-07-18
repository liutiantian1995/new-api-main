## ADDED Requirements

### Requirement: Admin 报表侧边栏菜单项

Admin 分组侧边栏 SHALL 在 "System Settings" 之后新增 "Reports" 菜单项，URL 指向 `/admin/reports`，仅对 role ≥ ADMIN (10) 的用户可见。

#### Scenario: 管理员看到 Reports 菜单

- **WHEN** role ≥ 10 的用户登录后查看侧边栏
- **THEN** Admin 分组底部出现 "Reports" 菜单项
- **AND** 点击后导航到 `/admin/reports`

#### Scenario: 普通用户看不到 Reports 菜单

- **WHEN** role < 10 的用户登录
- **THEN** 侧边栏 SHALL NOT 显示 "Reports" 菜单项
- **AND** 直接访问 `/admin/reports` URL 时 SHALL 被路由守卫拒绝并跳转

### Requirement: 报表页支持时间范围筛选

报表页顶部 SHALL 提供时间范围选择器，预设：今日、近 7 天、近 30 天、自定义区间。

#### Scenario: 切换时间范围

- **WHEN** 管理员选择"近 7 天"
- **THEN** 所有报表组件（趋势图、Top 排行、统计卡）SHALL 同步刷新为 7 天范围内的数据
- **AND** URL query 参数 SHALL 携带 `?range=7d` 以便分享

### Requirement: 报表页支持维度筛选

报表页 SHALL 提供筛选器：渠道（多选）、用户（多选）、分组（单选），所有筛选条件 AND 组合。

#### Scenario: 多渠道筛选

- **WHEN** 管理员勾选 channels = [1, 5, 8]
- **THEN** 所有报表数据 SHALL 只统计这三个渠道的消耗
- **AND** Top 排行的"渠道维度" SHALL 隐藏（因为已经按渠道筛选）

### Requirement: 报表页展示 Token 趋势图

报表页 SHALL 展示 24h/7d/30d 的 Token 趋势线图，包含 4 条线：Input、Cache Hit、Output、Total。

#### Scenario: 渲染趋势图

- **WHEN** 报表页加载完成
- **THEN** 趋势图 SHALL 显示 4 条不同颜色的线
- **AND** 鼠标 hover SHALL 显示该时间点的 4 个 token 数值
- **AND** 图例 SHALL 可点击切换显示/隐藏单条线

### Requirement: 报表页展示渠道 Top 排行

报表页 SHALL 展示按 token 消耗降序排列的渠道 Top 10 表格，列包含：渠道名称、请求数、Input Tokens、Cache Hit Tokens、Output Tokens、Total Tokens、消耗配额。

#### Scenario: 未筛选渠道时

- **WHEN** 报表页加载，未应用渠道筛选
- **THEN** 渠道 Top 10 表格 SHALL 显示
- **AND** 表格 SHALL 支持点击列头排序（默认按 Total Tokens 降序）

### Requirement: 报表页展示用户 Top 排行

报表页 SHALL 展示按 token 消耗降序排列的用户 Top 10 表格，列同渠道表（替换渠道名为用户名）。

#### Scenario: 渲染用户 Top 表格

- **WHEN** 报表页加载完成
- **THEN** 用户 Top 10 表格 SHALL 显示在渠道表下方
- **AND** 表格 SHALL 支持列头排序

### Requirement: Classic 主题暴露报表入口

Classic 主题侧边栏 SHALL 在 Admin 分组末尾显示 "Reports" 菜单项（i18n key 复用 default 主题），但点击后跳转到 default 主题的报表页（或显示 "请切换到默认主题" 提示）。

#### Scenario: Classic 主题用户点击 Reports

- **WHEN** classic 主题管理员点击 "Reports" 菜单
- **THEN** 系统 SHALL 跳转到 default 主题的 `/admin/reports`
- **OR** 显示提示："报表功能仅在默认主题可用"
