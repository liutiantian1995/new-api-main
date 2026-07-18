## Why

Token-aware channel routing (batch4) 已上线，但管理员无法从使用日志中看到每次请求的路由决策依据——是 tier boost 生效、默认 weight+priority、亲和命中还是 max_tokens 全空回退。当前 `logger.LogInfo` 只输出到容器 stdout，不落库，排障需要翻日志文件。需要在使用日志中增加一列「路由依据」，且仅管理员可见，方便运维诊断路由策略是否符合预期。

## What Changes

- 在 `middleware/distributor.go` 选渠道后计算路由决策依据（basis、est_tokens、base/effective priority、boost、fallback），存入 gin context
- 在 `model/log.go` `RecordConsumeLog` 中从 context 读取路由依据，写入 `Log.Other` JSON 字段的 `routing_info` key
- 在 `model/log.go` `formatUserLogs` 中为非管理员用户剥离 `routing_info`（复用现有 `admin_info`/`audit_info` 剥离模式）
- 前端 classic + default 主题日志表新增「路由依据」列，仅管理员可见
- 零 DB schema 变更：复用 `Log.Other` JSON 字段，不新增列、不 migration

## Capabilities

### New Capabilities

- `log-routing-basis`: 使用日志中的路由决策依据记录与展示。记录每次 API 请求的渠道选择决策路径（affinity / tier_boost / default / fallback）、估算 token 数、base/effective priority、boost 量、fallback 标记。仅管理员可在日志中查看，非管理员完全不感知。

### Modified Capabilities

（无——specs 目录为空，无既有能力需修改）

## Impact

- **后端**：
  - `constant/context_key.go` — 新增 `ContextKeyRoutingBasis`
  - `middleware/distributor.go` — 选渠道后计算 routing basis 存入 context
  - `model/log.go` — `RecordConsumeLog` 注入 `routing_info`；`formatUserLogs` 剥离非管理员字段
- **前端**：
  - `web/default/` — 日志表新增「路由依据」列（管理员可见）
  - `web/classic/` — 同上
- **DB**：无变更（复用 `Log.Other` JSON 字段）
- **API**：`/api/log/` 响应中 `other` 字段新增 `routing_info` key（仅管理员）
- **性能**：零额外开销（routing basis 在 distributor 中已计算，仅多一次 context set + JSON merge）
- **兼容性**：旧日志无 `routing_info` key，前端渲染时按空处理；非管理员 API 响应完全不包含该字段
