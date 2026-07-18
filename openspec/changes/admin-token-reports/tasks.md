## 1. 后端数据层（model/log.go 扩展）

- [x] 1.1 扩展 `Stat` 结构，增加 `PromptTokens`, `CompletionTokens`, `CachedTokens`, `TotalTokens`, `RequestCount` 字段
- [x] 1.2 修改 `SumUsedQuota` SQL，使用 `COALESCE(SUM(prompt_tokens), 0)` 等单独聚合，并保留 `tpm`/`rpm` 向后兼容字段
- [x] 1.3 实现跨 dialect 的 cache token 提取（SQLite `json_extract` / MySQL `JSON_EXTRACT` / PostgreSQL `::json->`，统一通过 `common.UsingMainDatabase` / `UsingLogDatabase` 分支）
- [x] 1.4 新增 `GetReportStats(startTime, endTime, channelIds, userIds, group)` 函数返回拆分的 token 统计
- [x] 1.5 新增 `GetTopChannels(startTime, endTime, limit)` 函数返回渠道 Top N
- [x] 1.6 新增 `GetTopUsers(startTime, endTime, limit)` 函数返回用户 Top N
- [x] 1.7 单元测试：覆盖 cache 字段缺失、不同 dialect 分支、limit 边界

## 2. 后端 controller + router

- [x] 2.1 扩展 `GetLogsStat` 返回拆分的 token 字段（`prompt_tokens`/`completion_tokens`/`cached_tokens`/`total_tokens`）
- [x] 2.2 新建 `controller/report.go`，实现 `GetReportStats`、`GetTopChannels`、`GetTopUsers` 三个 handler
- [x] 2.3 在 `router/api-router.go` 注册 `/api/report/stats`、`/api/report/top/channels`、`/api/report/top/users` 路由，全部走 `AdminAuth()` 中间件
- [ ] 2.4 集成测试：使用 httptest 验证权限（普通用户 403、admin 200）、参数校验、SQL 注入防护

## 3. 前端数据看板 token 拆分（default 主题）

- [x] 3.1 更新 `web/default/src/features/dashboard/hooks/use-dashboard-config.tsx` 的 `useSummaryCardsConfig`，扩展返回值支持 4 个 token 指标
- [x] 3.2 修改 `web/default/src/features/dashboard/components/overview/summary-cards.tsx`，布局改为 `md:grid-cols-4`，新增 Input/Cache/Output/Total 四卡
- [x] 3.3 更新 `useModelStatCardsConfig` 同步拆分（数据面板侧边栏）
- [x] 3.4 扩展 `web/default/src/features/dashboard/api.ts` 的 stat 响应类型，加入新字段
- [x] 3.5 验证 build：`cd web/default && bun run build`

## 4. 前端报表 feature 模块（default 主题）

- [ ] 4.1 创建目录 `web/default/src/features/reports/`，建立 `api.ts` / `types.ts` / `hooks/` / `components/` 子结构
- [ ] 4.2 实现 `api.ts`：`getReportStats`、`getTopChannels`、`getTopUsers` 三个 fetch 函数
- [ ] 4.3 实现 `types.ts`：`ReportStats`、`TopChannelRow`、`TopUserRow` 类型
- [ ] 4.4 实现 `components/report-filters.tsx`：时间范围选择器（今日/7d/30d/自定义）+ 渠道多选 + 用户多选 + 分组单选
- [ ] 4.5 实现 `components/token-trend-chart.tsx`：4 条线的趋势图（Input/Cache/Output/Total）
- [ ] 4.6 实现 `components/top-channels-table.tsx`：可排序表格，列含渠道/请求/各 token/配额
- [ ] 4.7 实现 `components/top-users-table.tsx`：同上但维度换为用户
- [ ] 4.8 实现 `index.tsx` 主页面，组装筛选器 + 趋势图 + 两张表，使用 React Query 拉取数据
- [ ] 4.9 路由注册：在 app router 中添加 `/admin/reports` 懒加载路由

## 5. 前端侧边栏注册（default 主题）

- [x] 5.1 修改 `web/default/src/hooks/use-sidebar-data.ts`，在 `admin.navGroups[0].items` 末尾追加 `{ title: t('Reports'), url: '/admin/reports', icon: BarChart3, requiredRole: ROLE.ADMIN }`
- [x] 5.2 修改 `use-sidebar-config.ts` 的 `URL_TO_CONFIG_MAP`，添加 `/admin/reports` → `{ section: 'admin', module: 'reports' }`
- [x] 5.3 修改 `DEFAULT_SIDEBAR_MODULES` 的 `admin` section，加入 `reports: true`
- [ ] 5.4 添加路由守卫：`/admin/reports` 路径仅 role ≥ ADMIN 可访问，否则重定向到 `/dashboard/overview`

## 6. i18n 与 classic 主题

- [x] 6.1 在 `web/default/src/i18n/locales/{en,zh}.json` 添加新 key：`Reports` / `Input Tokens` / `Cache Hit` / `Output Tokens` / `Time Range` / `Top Channels` / `Top Users` 等
- [ ] 6.2 classic 主题 `use-sidebar-data` 同步添加 Reports 入口（i18n key 复用），URL 指向 default 主题
- [ ] 6.3 classic i18n locales (`en/zh-CN/zh-TW`) 添加 Reports key

## 7. 测试与验证

- [ ] 7.1 后端单元测试：cache token 解析、top 排行 SQL、limit 边界
- [ ] 7.2 后端集成测试：3 个新 API endpoint 的权限 + 参数校验
- [ ] 7.3 前端组件测试：报表筛选器联动、Top 表格排序
- [ ] 7.4 E2E 验证：管理员能看到报表页、普通用户访问被拒、token 拆分卡片渲染正确
- [ ] 7.5 跨数据库验证：在 SQLite（默认）/ MySQL / PostgreSQL 上跑一次完整查询，确认 cache 提取 SQL 兼容

## 8. 文档与收尾

- [ ] 8.1 更新 `docs/superpowers/specs/` 下的设计文档（如有）
- [ ] 8.2 在 PR 描述中说明：新增报表页、token 拆分、向后兼容（`tpm`/`rpm` 字段保留）
- [ ] 8.3 标注 AI-generated（按 AGENTS.md 治理规则）
