## Why

当前数据看板的"总 Token 数"是 `prompt_tokens + completion_tokens` 的聚合，无法区分输入、缓存、输出各自的消耗占比。管理员也缺少一个集中的报表入口来按渠道/用户/时间维度分析 token 和请求量消耗，只能从分散的看板卡片中拼凑信息。

## What Changes

- **数据看板总览页**：将合并的"总 Token 数"拆分为输入 Token、缓存命中 Token、输出 Token 三个独立指标卡，保留总 Token 作为汇总。
- **新管理员侧边栏菜单项 "报表"(Reports)**：仅管理员可见（通过 `requiredRole: ROLE.ADMIN` 控制），位于 Admin 分组下 System Settings 之后。
- **报表页面**：支持按时间范围、渠道、用户、分组筛选，展示：
  - Token 消耗趋势（输入/缓存/输出/总量）
  - 渠道维度 Top 排行（token、quota、请求数）
  - 用户维度 Top 排行（token、quota、请求数）
  - 请求数 RPM / TPM 时序图
- **后端 API**：扩展 `GetLogsStat` 返回 prompt_tokens / completion_tokens / cached_tokens 分项；新增聚合查询接口用于渠道/用户 Top 排行。

## Capabilities

### New Capabilities
- `dashboard-token-breakdown`: 数据看板拆分输入/缓存/输出 token 指标
- `admin-reports`: 管理员报表侧边栏菜单项与页面
- `admin-reports-api`: 后端聚合统计 API（渠道/用户 Top 排行）

### Modified Capabilities
- (none) — 无现有 capability 需求变更

## Impact

- **后端**: `model/log.go` 扩展 `Stat` 结构（增加 `PromptTokens`, `CompletionTokens`, `CachedTokens`）、`SumUsedQuota`；`controller/log.go` 扩展 `GetLogsStat` 响应；`router/api-router.go` 新增报表 API 路由
- **前端 default**: `web/default/src/features/dashboard/` 新增 breakdown 卡片、新增 `reports/` 功能模块；`use-sidebar-data.ts` 添加 Admin 新菜单项；`use-dashboard-config.ts` 更新 summary 配置
- **前端 classic**: 按需同步侧边栏 i18n key（报表正文仍在 default 主题开发）
- **数据库**: 无 schema 变更（`logs` 表已有 `prompt_tokens`、`completion_tokens`；缓存数据从 response body 解析并持久化到新的 `cache_tokens` 列或实时计算）
