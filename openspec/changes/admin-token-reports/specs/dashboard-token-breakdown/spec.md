## ADDED Requirements

### Requirement: Dashboard summary shows token breakdown

数据看板首页 summary cards 区域 SHALL 展示四个 token 相关指标：输入 Token (Input)、缓存命中 Token (Cache Hit)、输出 Token (Output)、总 Token 数 (Total)，而不是单一的"总 Token 数"。

#### Scenario: 用户查看 overview dashboard

- **WHEN** 用户访问 `/dashboard/overview`
- **THEN** summary cards 区域展示 4 个 token 指标：Input Tokens、Cache Hit Tokens、Output Tokens、Total Tokens
- **AND** Total = Input + Output（Cache Hit 作为 Input 的子集单独标注，不重复计入 Total）

#### Scenario: Cache tokens 数据源

- **WHEN** 后端 `Log.Other` 字段包含 OpenAI 系 `usage.prompt_tokens_details.cached_tokens`
- **OR** Claude 系 `usage.cache_read_input_tokens` / `cache_creation_input_tokens`
- **THEN** 系统 SHALL 解析并累加为 Cache Hit Tokens 指标
- **AND** 缺失该字段的旧日志 SHALL 计为 0

### Requirement: Stat API 返回 token 拆分字段

`GetLogsStat` API 响应的 `data` 对象 SHALL 包含 `prompt_tokens`、`completion_tokens`、`cached_tokens`、`total_tokens` 四个字段，与现有 `quota`、`rpm`、`tpm` 字段并存。

#### Scenario: 调用 /api/log/stat

- **WHEN** 客户端 GET `/api/log/stat?type=2&start_timestamp=...&end_timestamp=...`
- **THEN** 响应 `data` 字段包含：
  - `quota` (int)
  - `rpm` (int)
  - `tpm` (int) — 总 token/min，保持向后兼容
  - `prompt_tokens` (int) — 输入 token 总数
  - `completion_tokens` (int) — 输出 token 总数
  - `cached_tokens` (int) — 缓存命中 token 总数（可能为 0）
  - `total_tokens` (int) — prompt + completion

### Requirement: Token 拆分卡片保留 Total 作为汇总

Total Tokens 卡片 SHALL 保持当前展示位置和语义不变（仍是 prompt + completion 的总和），新增三个拆分卡片作为子指标。

#### Scenario: Total 卡片与拆分卡片同时展示

- **WHEN** overview dashboard 渲染 token 指标区
- **THEN** Total Tokens 卡片 SHALL 显示在最前
- **AND** Input / Cache Hit / Output 三卡 SHALL 按此顺序排列在后
- **AND** 4 个卡片 SHALL 共享 `md:grid-cols-4` 网格布局
