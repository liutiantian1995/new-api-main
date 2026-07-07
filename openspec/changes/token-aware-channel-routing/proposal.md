## Why

当前渠道选择只看 `priority + weight + retry`，对请求大小"盲目"。当渠道池混合 200K 与 1M 上下文容量时，200K 上下文的大请求会随机命中 1M 渠道造成成本浪费，而 1M 大请求又可能命中 200K 渠道导致上游 413 失败。运维只能靠把大容量渠道单独分组来规避，缺乏细粒度、可配置的路由控制。

需要一种基于请求 token 数的动态路由偏好机制，让小请求偏好廉价小容量渠道、大请求自动流向大容量渠道，且不破坏现有 priority/weight 体系、不引入硬阻塞。

## What Changes

- 新增 Channel 字段 `max_tokens`（int）：渠道上下文硬上限，0=不限。请求估算 token 超过该值的渠道在路由时被软过滤（过滤后为空则回退到全集合）。
- 新增 Channel 字段 `token_tiers`（JSON 数组）：每条 `{max_tokens, priority_boost}` 表示"当请求估算 token ≤ max_tokens 时叠加 boost 到 base priority"。运行时计算 `effective_priority = base_priority + Σ(命中 tier 的 boost)`。
- 在 `middleware/distributor.go` 的 `Distribute` 中新增 token 估算步骤：复用 `common.GetBodyStorage(c)`，用 gjson 解析 OpenAI/Anthropic/Gemini/Embeddings 多协议的消息文本，按字符近似法（4字符≈1token，CJK 1.5字符≈1token）估算 input tokens，单次开销 <1ms。
- `model/channel_cache.go` 的 `GetRandomSatisfiedChannel` 签名新增 `estTokens int` 参数：先按 `max_tokens` 软过滤，再按 `effective_priority` DESC 分 tier，retry 跨 effective tier，tier 内按 weight 加权随机。
- Affinity 命中后增加 token 守卫：估算 token 超过 affinity 渠道 `max_tokens` 时弃用亲和，走正常 token 路由。
- `service/channel_select.go` 的 `CacheGetRandomSatisfiedChannel` 透传 estTokens；auto-group 跨组重试逻辑不变（priorityRetry 基于 effective_priority）。
- 前端（classic + default 双主题）渠道编辑页新增「Token 路由策略」折叠面板：max_tokens 输入框 + 可增删的 token_tiers 行（每行 max_tokens + priority_boost）。
- 数据库迁移：channels 表新增 `max_tokens` integer 列（default 0）与 `token_tiers` text 列（JSON，gorm serializer:json）。SQLite/MySQL/PostgreSQL 全兼容。
- 不修改计费、不上报估算值到日志（仅在 debug 级别输出），不修改 token 表结构。

## Capabilities

### New Capabilities
- `token-aware-routing`: 基于请求估算 token 数的渠道偏好路由——包含 token 估算、effective_priority 计算、max_tokens 软过滤、affinity token 守卫、降级回退

### Modified Capabilities
<!-- 无现有 spec 需修改 -->

## Impact

- **后端代码**：
  - `model/channel.go`：Channel struct 新增 2 字段
  - `model/channel_cache.go`：`GetRandomSatisfiedChannel` 签名与算法变更
  - `model/ability.go`：Ability 缓存加载时携带 token_tiers/max_tokens
  - `middleware/distributor.go`：Distribute 中新增估算 + 守卫
  - `service/channel_select.go`：RetryParam 新增 EstTokens 字段并透传
  - 新增 `service/token_estimate.go`：估算工具函数
- **数据库**：channels 表新增 2 列，AutoMigrate 自动处理，无需手写迁移脚本
- **前端**：
  - `web/classic/src/components/edit_channel/`：新增 Token 路由面板
  - `web/default/src/features/channels/`：同步新增
  - i18n 文案（en/zh 至少）
- **API 契约**：Channel CRUD 的 JSON 响应新增 `max_tokens`、`token_tiers` 字段，向后兼容（旧客户端忽略即可）
- **缓存**：`group2model2channels` 结构不变，token_tiers 随 channel 配置加载，无额外失效逻辑
- **性能**：单请求路由开销增加 ~0.5ms（估算），可忽略
- **不影响**：计费、日志结构、token 表、用户表、上游协议
