## ADDED Requirements

### Requirement: Report stats aggregation endpoint

后端 SHALL 提供 `GET /api/report/stats` 接口，接受 `start_timestamp`、`end_timestamp`、`channel_ids`（逗号分隔）、`user_ids`（逗号分隔）、`group` 参数，返回该范围内的聚合统计。

#### Scenario: 调用 stats 接口

- **WHEN** 客户端 GET `/api/report/stats?start_timestamp=...&end_timestamp=...`
- **THEN** 响应 `data` 字段 SHALL 包含：
  - `prompt_tokens` (int)
  - `completion_tokens` (int)
  - `cached_tokens` (int)
  - `total_tokens` (int) — prompt + completion
  - `quota` (int)
  - `request_count` (int)
- **AND** 该接口 SHALL 仅对 role ≥ ADMIN 的用户开放（`AdminAuth()` 中间件）

#### Scenario: 应用筛选条件

- **WHEN** 请求包含 `channel_ids=1,5`
- **THEN** 查询 SHALL 只统计 `channel_id IN (1, 5)` 的日志
- **AND** 多个筛选条件 SHALL 按 AND 组合

### Requirement: Top channels aggregation endpoint

后端 SHALL 提供 `GET /api/report/top/channels` 接口，按时间范围和可选筛选条件返回渠道 Top N 排行。

#### Scenario: 调用 top channels 接口

- **WHEN** 客户端 GET `/api/report/top/channels?start_timestamp=...&end_timestamp=...&limit=10`
- **THEN** 响应 `data` SHALL 为数组，每项包含：
  - `channel_id` (int)
  - `channel_name` (string)
  - `prompt_tokens` (int)
  - `completion_tokens` (int)
  - `cached_tokens` (int)
  - `total_tokens` (int)
  - `quota` (int)
  - `request_count` (int)
- **AND** 数组 SHALL 按 `total_tokens` 降序排列

#### Scenario: limit 参数边界

- **WHEN** `limit` 参数省略或 ≤ 0
- **THEN** 系统 SHALL 默认 `limit = 10`
- **WHEN** `limit` > 100
- **THEN** 系统 SHALL 截断为 100

### Requirement: Top users aggregation endpoint

后端 SHALL 提供 `GET /api/report/top/users` 接口，语义同 top channels，按用户维度聚合。

#### Scenario: 调用 top users 接口

- **WHEN** 客户端 GET `/api/report/top/users?start_timestamp=...&end_timestamp=...&limit=10`
- **THEN** 响应 `data` SHALL 为数组，每项包含：
  - `user_id` (int)
  - `username` (string)
  - `prompt_tokens` / `completion_tokens` / `cached_tokens` / `total_tokens` / `quota` / `request_count`
- **AND** 数组 SHALL 按 `total_tokens` 降序排列

### Requirement: Cache tokens 解析逻辑

后端 SHALL 在所有报表聚合查询中，从 `logs.other` JSON 字段提取 cache token 信息，归一化为单一 `cached_tokens` 值。

#### Scenario: OpenAI 系日志

- **WHEN** `logs.other` 包含 `usage.prompt_tokens_details.cached_tokens`
- **THEN** 该值 SHALL 计入 `cached_tokens`

#### Scenario: Claude 系日志

- **WHEN** `logs.other` 包含 `usage.cache_read_input_tokens` 和/或 `usage.cache_creation_input_tokens`
- **THEN** 两者之和 SHALL 计入 `cached_tokens`

#### Scenario: 缺失 cache 字段的日志

- **WHEN** `logs.other` 不包含上述任何字段
- **THEN** `cached_tokens` SHALL 计为 0
- **AND** 查询 SHALL NOT 报错

### Requirement: Cross-database compatibility

所有报表查询 SHALL 同时支持 SQLite、MySQL ≥ 5.7.8、PostgreSQL ≥ 9.6。

#### Scenario: 不同 dialect 的 JSON 提取

- **WHEN** 在 SQLite 上运行查询
- **THEN** 系统 SHALL 使用 `json_extract(other, '$.usage...')` 提取 cache 字段
- **WHEN** 在 MySQL 上运行查询
- **THEN** 系统 SHALL 使用 `JSON_EXTRACT(other, '$.usage...')`
- **WHEN** 在 PostgreSQL 上运行查询
- **THEN** 系统 SHALL 使用 `other::json->>'usage'` 或 `->` 路径表达式

如果跨 dialect JSON 函数差异过大，可在 Go 层做行级解析（取出 `Other` 字符串后 `common.Unmarshal`），代价是查询时需扫描完整行。
