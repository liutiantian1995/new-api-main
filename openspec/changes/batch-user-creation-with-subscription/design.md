## Context

new-api 是一个 AI API 网关，现有的用户管理只能逐个创建用户，订阅管理是即时生效的。本次二次开发在保持原有项目架构（Router -> Controller -> Service -> Model 分层）和处理风格的前提下，新增批量创建用户和订阅延迟生效能力。

### 现状
- 用户创建：`controller/user.go::CreateUser` -> `model/user.go::User.Insert`，单条创建。
- 订阅绑定：`controller/subscription.go::AdminCreateUserSubscription` -> `model/subscription.go::AdminBindSubscription` -> `CreateUserSubscriptionFromPlanTx`，立即设置 `StartTime` 和 `EndTime`，状态为 `active`。
- 令牌创建：`model/token.go::Token.Insert`，单条创建。
- 登录流程：`controller/user.go::Login` -> `setupLogin`，更新 `last_login_at`。
- 数据库兼容：需同时支持 SQLite / MySQL / PostgreSQL。

### 约束
- 不改动存量业务逻辑，仅在现有接口上扩展。
- 保护项目标识（nеw-аρi / QuаntumΝоuѕ）不可修改。
- 遵循 `common/json.go` 封装、GORM 跨数据库规范、i18n 规范。

## Goals / Non-Goals

**Goals:**
- 提供批量创建用户的后端 API（单事务，支持前缀+日期后缀+序号生成用户名，密码为 `用户名@123`）。
- 支持批量设置分组、批量关联订阅套餐、批量创建 API 密钥。
- 引入订阅生效策略字段（`immediate` / `on_use`）。
- 在 `on_use` 策略下，订阅创建时状态为 `pending`，计时起点延迟到用户首次登录或首次使用令牌调用 API。
- 提供批量创建用户的前端页面，风格与现有用户管理一致。
- 兼容 SQLite / MySQL / PostgreSQL。

**Non-Goals:**
- 不修改现有单用户创建、订阅购买、令牌管理的核心逻辑。
- 不引入新的支付渠道或计费模型。
- 不重构现有数据库表结构（仅新增字段，不改动现有字段语义）。
- 不实现批量删除用户、批量导入用户（CSV/Excel）等扩展能力。

## Decisions

### 决策 1：批量创建 API 形态——单端点 + 事务

**选择**：新增 `POST /api/user/batch` 单端点，在单个数据库事务内完成所有用户、订阅、令牌的创建。

**理由**：
- 与现有 `POST /api/user/`（单用户创建）风格一致。
- 单事务保证原子性：要么全部成功，要么全部回滚。
- 避免多次 HTTP 往返。

**备选方案**：异步任务队列 + 进度查询。被否决，因为批量规模通常在数十到数百级别，同步处理足够；异步引入额外复杂度。

### 决策 2：用户名生成规则——前缀 + 日期后缀 + 序号

**选择**：用户名格式为 `{prefix}{date}{seq}`，例如 `user060101`、`user060102`。
- `prefix`：管理员指定（必填，1-10 字符）。
- `date`：默认 `MMDD` 格式（如 `0601`），可由前端配置为空。
- `seq`：从 1 开始的序号，按批量数量补零（例如批量 50 个则 2 位序号）。
- 密码：`{username}@123`。

**理由**：
- 满足用户提出的"前缀 + 0601 这种默认日期 + 密码用户名@123"需求。
- 序号补零保证可排序、可读。

**备选方案**：UUID/随机字符串。被否决，用户明确要求前缀+日期+序号的可读命名。

### 决策 3：订阅生效策略——通过新增表实现，原表零修改

**选择**：不修改 `UserSubscription` 表，新增 `pending_subscription_activations` 表：

```
pending_subscription_activations
  id                    (PK)
  user_id               (INT, INDEX)
  user_subscription_id  (INT, FK -> user_subscriptions.id, INDEX)
  plan_id               (INT, INDEX)
  activation_strategy   (varchar(16), default 'on_use')  -- 生效策略
  status                (varchar(16), default 'pending')  -- pending/activated
  activated_at          (bigint, default 0)               -- 实际激活时间
  created_at            (bigint)
```

**数据流**：
1. `on_use` 策略创建订阅时：
   - `user_subscriptions` 表：`status='pending'`, `start_time=0`, `end_time=0`（仅复用 Status 字段，不新增字段）
   - `pending_subscription_activations` 表：插入一行 `status='pending'`，记录 `user_id`/`plan_id`/`activation_strategy`
2. 激活时（登录或令牌调用触发）：
   - 从事务中 `UPDATE user_subscriptions SET status='active', start_time=当前时间, end_time=套餐时长后` WHERE `id=当前行`
   - 同步 `UPDATE pending_subscription_activations SET status='activated', activated_at=当前时间` WHERE `id=当前行`

**状态约定**：
- `UserSubscription.Status` 扩展语义：在原有 `active`/`expired`/`cancelled` 之外新增 `pending`，**不新增字段**
- `pending_subscription_activations.status`：`pending`（待激活）→ `activated`（已激活）

**理由**：
- 原 `UserSubscription` 表零新增字段，不触碰核心表的迁移风险
- `pending_subscription_activations` 是纯增量子表，删除后不影响现有逻辑
- 现有订阅查询（`WHERE status='active' AND end_time > now`）对 pending 行自然过滤，零改动兼容
- 激活逻辑清晰：user_subscriptions 承载计费周期，pending 表承载激活队列

**备选方案 A**：在 `UserSubscription` 表新增 `activation_strategy`/`activated_at` 字段。被否决，原表零修改更安全。
**备选方案 B**：仅在 `pending_subscription_activations` 表维护全部信息，原 `UserSubscription` 保持 active 但 start_time=0。被否决，会导致现有订阅计费逻辑紊乱（`PreConsumeUserSubscription` 等依赖 start_time/end_time 判断激活状态）。

### 决策 4：存量接口的 on_use 入口——可选参数扩展

**选择**：在存量 `AdminCreateUserSubscription` 控制器和 `AdminBindSubscription` model 方法中增加可选的 `activation_strategy` 参数（默认 `immediate`），不传该参数则行为与变更前完全一致。

**前端改动**：在"用户管理 → 用户订阅管理"弹窗中增加"生效策略"下拉选项（`immediate`/`on_use`），仅在前端表单中新增，不影响现有管理流程的数据结构和接口签名。

**on_use 进入路径汇总**：
1. **批量创建用户 API**（新增）：`POST /api/user/batch`，支持 `activation_strategy` 参数
2. **用户订阅管理弹窗**（存量扩展）：`POST /subscription/admin/users/:id/subscriptions`，新增可选 `activation_strategy` 参数
3. **管理员直接绑定接口**（存量扩展）：`POST /subscription/admin/bind`，新增可选 `activation_strategy` 参数

以上三个入口均通过新增的 `AdminBindSubscriptionWithStrategy` 统一处理，保证逻辑一致性。

**理由**：
- 存量接口保持向后兼容：不传 `activation_strategy` 等同于 `immediate`
- 统一入口：三个路径都调用同一 model 方法，减少代码重复
- 前端仅在表单中新增选项，不改动现有 UI 结构

### 决策 5：激活逻辑的幂等性

**选择**：`ActivatePendingSubscriptions` 使用条件更新保证幂等：
1. 从 `pending_subscription_activations` 表查询 `status='pending'` 的行
2. 事务内：UPDATE `user_subscriptions` SET status='active', start_time=当前时间, end_time=套餐时长后，同时 UPDATE `pending_subscription_activations` SET status='activated', activated_at=当前时间
3. 两表更新在同一事务内完成，任一失败回滚

### 决策 6：批量创建的事务边界

**选择**：单个 `DB.Transaction` 包裹：
1. 循环创建用户（含密码哈希、分组设置）。
2. 循环为每个用户创建订阅（若指定套餐）。
3. 循环为每个用户创建令牌（若指定）。

任一用户创建失败，整个事务回滚。返回成功创建的用户列表和失败原因（若有，但因事务回滚通常为空或全部成功）。

**理由**：原子性优先，符合"批量"语义。部分成功会导致数据不一致和管理员困惑。

### 决策 7：数据库迁移

**选择**：通过 GORM AutoMigrate 新建 `pending_subscription_activations` 表，不修改任何现有表。GORM 的 `AutoMigrate` 对新建表支持完全兼容三种数据库。

**理由**：
- 新建表零迁移风险：不涉及 ALTER TABLE 添加列的兼容性问题
- 不影响现有表结构：所有 `user_subscriptions` 相关查询、索引、缓存逻辑零改动
- 回滚简单：只需 DROP 新表即可完全恢复

## Risks / Trade-offs

- **[批量创建性能]** 大批量（>500）用户创建在单事务内可能耗时较长。→ 限制单次批量上限（如 200），超过则返回错误提示分批。
- **[用户名冲突]** 批量创建时用户名可能与现有用户冲突。→ 创建前预检查 + 事务内唯一约束兜底，冲突时整个事务回滚并返回冲突的用户名。
- **[订阅激活竞态]** 登录和 API 调用同时触发激活。→ 使用 `WHERE status = 'pending'` 条件更新 `pending_subscription_activations` 表，数据库行锁保证幂等。
- **[密码强度]** `{username}@123` 密码强度较低。→ 这是管理员批量初始化场景，预期用户首次登录后修改密码；在返回结果中提示密码规则。
- **[现有令牌创建逻辑]** 现有 `Register` 中在用户创建后生成默认令牌的逻辑是内联的，不便于复用。→ 提取为独立函数 `CreateDefaultTokenForUser`，但保持原有调用点不变（仅在批量创建中调用新函数）。

## Migration Plan

1. **数据库迁移**：部署后 GORM AutoMigrate 自动创建 `pending_subscription_activations` 新表，对现有 `user_subscriptions` 表零修改，对现有订阅无影响。
2. **后端部署**：新增 API 端点，不影响现有端点。
3. **前端部署**：新增批量创建入口，不影响现有用户管理页面。
4. **回滚**：若需回滚，移除新增 API 路由和前端入口，DROP `pending_subscription_activations` 表即可完全恢复。原 `user_subscriptions` 表因零修改无需回滚。

## Open Questions

- 批量创建上限：建议 200，需确认。
- 用户名日期格式：默认 `MMDD`，是否需要支持 `YYYYMMDD`？
- `on_use` 策略下，订阅是否有有效期（即创建后多久未激活自动失效）？当前设计无自动失效，需确认。
