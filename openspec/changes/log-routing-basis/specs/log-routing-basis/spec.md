## ADDED Requirements

### Requirement: 路由依据计算与记录

系统 SHALL 在每次 API 请求完成渠道选择后，计算路由决策依据并记录到使用日志的 `Other.routing_info` 字段。

routing_info 结构 MUST 包含以下字段：
- `basis`: 决策路径类型，取值为 `affinity` | `tier_boost` | `default` | `fallback`
- `est_tokens`: 估算输入 token 数（0 表示未估算/非文本路径）
- `base_priority`: 选中渠道的 base priority
- `effective_priority`: 选中渠道的 effective priority（base + Σ boost，affinity 路径下等于 base_priority）
- `boost`: 生效的 boost 总量（= effective_priority - base_priority，可为负）
- `fallback`: 是否因 max_tokens 全空回退到全集合

basis 判定优先级（先匹配先返回）：
1. `fallback`: 当 `fallback == true` 时
2. `affinity`: 当渠道通过亲和命中分支选中时
3. `tier_boost`: 当 `estTokens > 0` 且 `effective_priority > base_priority` 时
4. `default`: 其余所有情况

#### Scenario: tier boost 生效时记录正确

- **WHEN** estTokens=1000，选中渠道 base_priority=10，token_tiers 配置 `{max_tokens:50000, priority_boost:5}` 命中，effective_priority=15
- **THEN** routing_info.basis = "tier_boost"，boost = 5，fallback = false

#### Scenario: 默认 weight+priority 路由

- **WHEN** estTokens=0（非文本路径或未估算），选中渠道 base_priority=10，无 token_tiers 配置
- **THEN** routing_info.basis = "default"，effective_priority = 10，boost = 0，fallback = false

#### Scenario: max_tokens 全空回退

- **WHEN** estTokens=500000，所有候选渠道 max_tokens 均被超限，fallback=true，从全集合选中渠道 base_priority=10
- **THEN** routing_info.basis = "fallback"，fallback = true（即使该渠道有 token_tiers，basis 仍为 fallback）

#### Scenario: 亲和命中

- **WHEN** 用户有上次渠道亲和且未超 max_tokens，直接选中亲和渠道（未走 CacheGetRandomSatisfiedChannel）
- **THEN** routing_info.basis = "affinity"，boost = 0，effective_priority = base_priority，fallback = false

#### Scenario: 非文本路径不估算 token

- **WHEN** 请求路径为 MJ/Suno/audio/images（estTokens=0），走默认 base priority + weight 路由
- **THEN** routing_info.basis = "default"，est_tokens = 0

### Requirement: 管理员独占可见性

系统 SHALL 确保 `routing_info` 字段仅对管理员用户可见，非管理员用户获取日志时该字段 MUST 被完全剥离。

剥离实现 MUST 复用 `formatUserLogs` 现有的 admin-only 字段删除模式（与 `admin_info`/`audit_info`/`is_model_mapped` 同等处理）。

#### Scenario: 非管理员用户查询日志不返回 routing_info

- **WHEN** 普通用户调用 `GET /api/log/self` 或 `GET /api/log/token/{tokenId}`
- **THEN** 响应中每条日志的 `other` 字段不包含 `routing_info` key（完全删除，非空值）

#### Scenario: 管理员查询日志保留 routing_info

- **WHEN** 管理员调用 `GET /api/log/`（GetAllLogs 路径，不经过 formatUserLogs）
- **THEN** 响应中每条日志的 `other` 字段保留完整 `routing_info` 对象

#### Scenario: 旧日志无 routing_info 字段

- **WHEN** 查询 batch4 之前产生的旧日志（`Other` 中无 `routing_info` key）
- **THEN** 管理员看到的 `other` 字段不含 `routing_info`（按空处理），非管理员也无差异

### Requirement: 前端日志表展示路由依据

前端 classic 与 default 主题的日志列表页 SHALL 新增「路由依据」列，仅管理员用户可见该列。

列内容 MUST 展示 basis 类型的可读标签（如「tier_boost」「默认」「回退」「亲和」），并在 hover/展开时显示详细 routing_info（est_tokens、base/effective priority、boost）。

非管理员用户 MUST NOT 看到该列（列本身不渲染，而非渲染后隐藏）。

#### Scenario: 管理员看到路由依据列

- **WHEN** 管理员用户打开使用日志页面
- **THEN** 日志表显示「路由依据」列，每行显示 basis 对应的可读标签

#### Scenario: 非管理员看不到路由依据列

- **WHEN** 普通用户打开使用日志页面
- **THEN** 日志表不显示「路由依据」列（列定义不存在，非 CSS 隐藏）

#### Scenario: 旧日志显示未记录

- **WHEN** 管理员查看旧日志（routing_info 为空）
- **THEN** 「路由依据」列显示「-」或「未记录」占位符
