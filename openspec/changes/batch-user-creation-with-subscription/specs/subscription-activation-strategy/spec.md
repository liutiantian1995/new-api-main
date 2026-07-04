## ADDED Requirements

### Requirement: 待激活订阅记录表

系统 SHALL 新增 `pending_subscription_activations` 表，记录每条需要延迟激活的订阅。每条 `on_use` 策略创建的订阅 MUST 在该表中有对应一行，`status` 为 `pending`。

#### Scenario: on_use 订阅创建时同步写入

- **WHEN** 管理员通过 `AdminBindSubscriptionWithStrategy` 为用户创建 `on_use` 订阅
- **THEN** 同时在 `pending_subscription_activations` 表插入一行，`status='pending'`，`activation_strategy='on_use'`，`activated_at=0`

#### Scenario: immediate 订阅创建时不写入该表

- **WHEN** 管理员创建 `immediate` 策略的订阅（含现有 `AdminBindSubscription` 调用）
- **THEN** 不在 `pending_subscription_activations` 表写入任何记录，行为与变更前完全一致

### Requirement: UserSubscription 状态扩展

系统 SHALL 支持 `UserSubscription.Status` 取值为 `pending`，表示订阅因 `on_use` 策略尚未激活。`pending` 状态的订阅 MUST 有 `StartTime=0`、`EndTime=0`（仅复用 Status 字段，**不新增字段**）。

#### Scenario: pending 状态订阅被计费逻辑自然过滤

- **WHEN** 计费预消费逻辑查询用户的活跃订阅（`WHERE status='active' AND end_time > now`）
- **THEN** `pending` 状态的订阅被自动过滤，不会参与额度扣减

### Requirement: 使用时激活订阅

系统 SHALL 在用户首次登录或首次使用 API 密钥时（两者任一即可），激活该用户所有 `pending` 状态的订阅。激活操作 MUST 幂等。

#### Scenario: 登录时激活

- **WHEN** 用户通过任意登录方式（密码、OAuth、2FA、Passkey）登录成功
- **THEN** 系统调用 `ActivatePendingSubscriptions(userId)`：
  1. 从 `pending_subscription_activations` 表查询该用户 `status='pending'` 的行
  2. 对每行，在事务内 UPDATE `user_subscriptions` SET `status='active'`、`StartTime=当前时间`、`EndTime=套餐时长后`（重新计算）
  3. 同步 UPDATE `pending_subscription_activations` SET `status='activated'`、`activated_at=当前时间`
  4. 若查询无 `pending` 行，立即返回，不走事务

#### Scenario: API 密钥首次使用时激活

- **WHEN** 用户通过 API 密钥发起请求，令牌验证中间件检测到 `pending_subscription_activations` 表中该用户存在 `status='pending'` 的行
- **THEN** 系统激活这些订阅（同登录时激活逻辑），并在 Redis 中标记该用户已激活，避免重复查询

#### Scenario: 已激活订阅不重复激活

- **WHEN** 用户再次登录或再次调用 API，且 `pending_subscription_activations` 表中该用户已无 `status='pending'` 的行
- **THEN** 系统不执行任何事务更新（`WHERE status='pending'` 查询返回空即返回），不影响登录或 API 调用性能

#### Scenario: 两处同时触发

- **WHEN** 用户同时通过登录和 API 密钥触发激活
- **THEN** `WHERE status='pending'` 条件更新保证行级锁互斥，仅一方实际执行激活操作，另一方查询到 0 行后直接返回

### Requirement: 批量创建时支持生效策略

系统 SHALL 在批量创建用户 API 中支持 `activation_strategy` 参数，为批量创建的用户的订阅指定生效策略。

#### Scenario: 批量创建 on_use 订阅

- **WHEN** 管理员通过 `POST /api/user/batch` 创建用户，指定 `plan_id=1`、`activation_strategy=on_use`
- **THEN** 为每个用户创建：
  - `user_subscriptions`：`status='pending'`，`start_time=0`，`end_time=0`
  - `pending_subscription_activations`：`status='pending'`，`activation_strategy='on_use'`

#### Scenario: 批量创建 immediate 订阅

- **WHEN** 管理员通过 `POST /api/user/batch` 创建用户，指定 `plan_id=1`，`activation_strategy` 为 `immediate` 或未指定
- **THEN** 为每个用户创建 `user_subscriptions`：`status='active'`，`StartTime=创建时间`、`EndTime` 按套餐时长计算
- **AND** 不写入 `pending_subscription_activations` 表

### Requirement: 现有 AdminBindSubscription 兼容性

系统 SHALL 保持现有 `AdminBindSubscription(userId, planId, sourceNote)` 签名的兼容性，默认使用 `immediate` 策略，不写入 `pending_subscription_activations` 表。

#### Scenario: 现有调用不受影响

- **WHEN** 通过现有 `AdminCreateUserSubscription` 控制器调用 `AdminBindSubscription(userId, planId, "")`，不传 `activation_strategy` 或传 `immediate`
- **THEN** 行为与变更前完全一致：`user_subscriptions.status='active'`，无 `pending_subscription_activations` 记录

#### Scenario: 用户订阅管理弹窗创建 on_use 订阅

- **WHEN** 管理员在"用户管理 → 用户订阅管理"弹窗中创建订阅，选择生效策略为 `on_use`
- **THEN** 请求携带 `activation_strategy=on_use`，后端调用 `AdminBindSubscriptionWithStrategy`，创建 `user_subscriptions.status='pending'` + `pending_subscription_activations.status='pending'`

#### Scenario: 管理员直接绑定接口创建 on_use 订阅

- **WHEN** 调用 `POST /subscription/admin/bind` 并指定 `activation_strategy=on_use`
- **THEN** 订阅按 `on_use` 策略创建为 `pending` 状态

#### Scenario: 新增带策略的绑定

- **WHEN** 调用新增的 `AdminBindSubscriptionWithStrategy(userId, planId, strategy, sourceNote)`，`strategy=on_use`
- **THEN** `user_subscriptions.status='pending'`，并写入 `pending_subscription_activations` 表
