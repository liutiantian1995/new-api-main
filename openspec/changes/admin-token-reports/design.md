## Context

当前数据看板 overview 的 summary-cards 使用 `model.SumUsedQuota`，它从 `logs` 表聚合 `quota` / `count(*)` / `prompt_tokens + completion_tokens` 三个值。日志表已有 `prompt_tokens` 和 `completion_tokens` 两列，但没有独立的 cache token 列——OpenAI/Claude 的上游响应里 cache hit 数据嵌套在 `usage` 对象中（如 `prompt_tokens_details.cached_tokens`），并未单独落库。

前端数据看板的 token 展示在 `use-dashboard-config.ts` 的 `useModelStatCardsConfig`（侧边栏数据面板）和 `summary-cards.tsx`（首页概览）两处。侧边栏结构定义在 `use-sidebar-data.ts`，支持按 `section='admin'` 分组菜单项。

管理员报表是全新页面，需要：
1. 后端聚合 API（渠道/用户 Top 排行 + token 拆分）
2. 侧边栏路由注册
3. 报表 UI（筛选 + 图表 + 表格）

## Goals / Non-Goals

**Goals:**
- 看板首页 `summary-cards.tsx` 将 token 拆为输入 / 缓存 / 输出 / 总 四卡
- 管理员侧边栏新增 "报表" 入口，仅 ADMIN 及以上角色可见
- 报表页支持按时间 / 渠道 / 用户 / 分组筛选，展示 token 消耗趋势 + Top 排行
- 后端 `Stat` 结构扩展，`GetLogsStat` 返回拆分的 token 字段

**Non-Goals:**
- 不改变现有的计费 / 配额逻辑（只读展示）
- 不做实时流式报表（定时刷新即可）
- 不单独持久化 cache token 列（从 `Other` JSON 字段实时提取，避免 schema 变更）
- Classic 主题不做完整的报表页面，仅在侧边栏暴露入口（i18n key）

## Decisions

### D1: Cache token 从 `Other` JSON 实时提取，不新增 DB 列

上游 relay 已在 `Log.Other` 字段中存储了完整的 response body 解析结果，包括 `usage.prompt_tokens_details.cached_tokens`（OpenAI 系）和 `usage.cache_creation_input_tokens` / `cache_read_input_tokens`（Claude 系）。

方案 A（新增 `cache_tokens` 列）需要 ALTER TABLE + 回填历史数据，且每种上游格式的 cache 字段不同。
方案 B（从 `Other` 实时解析）零 schema 变更，聚合时按需计算。

**选 B**。单条 log 的 Other 字段已有索引命中率，聚合 UDF 在 SQLite/MySQL/PG 上都可实现。代价是 SUM 查询比多扫一列文本略长，但数据量级下可接受。

### D2: 报表 API 设计为 `/api/report/stats` 前缀

- `GET /api/report/stats?start=...&end=...&channel=...&user=...&group=...` → Token 拆分汇总 + RPM/TPM
- `GET /api/report/top/channels?start=...&end=...&limit=10` → 渠道 Top N（按 token / quota / 请求数）
- `GET /api/report/top/users?start=...&end=...&limit=10` → 用户 Top N

所有接口走 `AdminAuth()` 中间件。复用现有 `logs` 表索引（`idx_created_at_type`、`idx_user_id_id` 等）。

### D3: 前端报表页放在 `web/default/src/features/reports/`

独立 feature 模块，结构参照 `dashboard/`（hooks + components + api + types）。不串入 dashboard bundle，首次访问时按需加载（`React.lazy`）。

### D4: 侧边栏注册位置与角色控制

在 `use-sidebar-data.ts` 的 `admin.navGroups` 数组末尾追加一项：
```
{ title: t('Reports'), url: '/admin/reports', icon: BarChart3, requiredRole: ROLE.ADMIN }
```
注意这里 `requiredRole: ROLE.ADMIN`（role ≥ 10），确保仅管理员可见。

### D5: 看板 token 拆分展示策略

- 24h 概览保留 "总 Token 数" 汇总卡
- 新增 "输入 Token" / "缓存命中" / "输出 Token" 三个子卡
- 布局由 `md:grid-cols-3` 改为 `md:grid-cols-4`
- 数据源：扩展 `Stat` 结构增加 `PromptTokens / CompletionTokens / CachedTokens`

## Risks / Trade-offs

- **[聚合查询性能 Top 排行]** → 复用 `logs` 表已有 composite index `idx_created_at_type` + `idx_created_at_id`；channel/user 维度走 `channel_id`/`user_id` 单列索引。
- **[Other 字段解析 JSON 开销]** → SQLite/MySQL/PG 均有原生 JSON 提取函数；`CachedTokens` 仅在报表 API 内部计算，不污染核心 stat 响应。
- **[Classic 主题不一致]** → 报表 UI 仅 default 主题提供；classic 通过 i18n key 暴露入口，但页面内容由 default 主题承载（未来可补 classic）。

## Open Questions

1. **Cache token 是否需要独立的 DB 列** — 当前决策为否（从 Other 实时解析），但如未来报表查询性能出现瓶颈，可聚合成 materialized view 或新增列。
2. **报表页是否支持导出 CSV/Excel** — 不在首版范围；可作为后续 PR。
