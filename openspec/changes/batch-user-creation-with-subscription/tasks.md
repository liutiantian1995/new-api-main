## 1. 数据库与模型层

- [x] 1.1 新增 `PendingSubscriptionActivation` 结构体定义（`model/subscription.go`），字段：`Id`/`UserId`/`UserSubscriptionId`/`PlanId`/`ActivationStrategy`/`Status`/`ActivatedAt`/`CreatedAt`
- [x] 1.2 在 `model/main.go` 的 `InitTable` 中通过 `AutoMigrate` 创建 `pending_subscription_activations` 新表（不修改 `user_subscriptions` 表）
- [x] 1.3 修改 `CreateUserSubscriptionFromPlanTx` 支持 `strategy` 参数：当 `strategy=on_use` 时，`user_subscriptions` 创建为 `status='pending'`、`start_time=0`、`end_time=0`，并在 `pending_subscription_activations` 表同步插入一行 `status='pending'`
- [x] 1.4 新增 `AdminBindSubscriptionWithStrategy(userId, planId int, strategy, sourceNote string)` 方法（`model/subscription.go`），内部调用修改后的 `CreateUserSubscriptionFromPlanTx`
- [x] 1.5 新增 `ActivatePendingSubscriptions(userId int) error` 方法（`model/subscription.go`）：查询 `pending_subscription_activations` 中 `status='pending'` 的行，在事务内 UPDATE `user_subscriptions`（status=active, start_time, end_time）+ UPDATE `pending_subscription_activations`（status=activated, activated_at）
- [x] 1.6 新增 `HasPendingSubscriptions(userId int) (bool, error)` 方法，用于令牌验证中间件快速判断是否需要激活

## 2. 批量创建用户核心逻辑

- [x] 2.1 在 `model/user.go` 中新增 `BatchCreateUsers` 方法，接收前缀、日期、数量、分组、角色、套餐 ID、生效策略、是否创建令牌等参数
- [x] 2.2 实现用户名生成逻辑：`{prefix}{date}{seq}`，序号按数量补零
- [x] 2.3 实现密码生成：`{username}@123`，使用 `common.Password2Hash` 哈希后存储
- [x] 2.4 在事务内循环调用 `User.Insert` 创建所有用户
- [x] 2.5 若指定了套餐 ID，循环调用 `AdminBindSubscriptionWithStrategy` 为每个用户创建订阅
- [x] 2.6 若启用令牌创建，提取 `CreateDefaultTokenForUser` 函数并循环调用
- [x] 2.7 预检查用户名冲突：在事务开始前批量查询可能的用户名是否已存在

## 3. Controller 与路由

- [x] 3.1 在 `controller/user.go` 中新增 `BatchCreateUsersRequest` 结构体定义请求参数
- [x] 3.2 在 `controller/user.go` 中新增 `BatchCreateUsers` 处理函数，包含参数校验、调用 model 层、返回结果
- [x] 3.3 在 `router/api-router.go` 中注册 `POST /api/user/batch` 路由，绑定 `middleware.AdminAuth()`
- [x] 3.4 修改 `controller/user.go::setupLogin`，登录成功后调用 `model.ActivatePendingSubscriptions(userId)`

## 4. 使用时激活集成

- [x] 4.1 在 `controller/user.go::setupLogin` 中，登录成功后异步调用 `model.ActivatePendingSubscriptions(userId)`（不阻塞登录流程）
- [x] 4.2 在令牌验证中间件（TokenAuth）中，通过 Redis 标志位（`user:{id}:subs_activated`）判断是否已激活；未激活时调用 `HasPendingSubscriptions` + `ActivatePendingSubscriptions`，激活后写入标志位

## 5. 前端批量创建页面

- [x] 5.1 新增 `web/default/src/features/users/components/batch-create-drawer.tsx` 组件，包含表单：前缀、日期后缀、数量、分组选择、套餐选择、生效策略选择、是否创建令牌
- [x] 5.2 修改 `web/default/src/features/users/index.tsx`，增加"批量创建"按钮入口
- [x] 5.3 新增前端 API 调用函数 `batchCreateUsers`（`web/default/src/features/users/api.ts`）
- [x] 5.4 添加 i18n 翻译 key（中/英）：批量创建相关文案

## 6. 管理与运维

- [x] 6.1 新增 `user.batch_create` 审计日志类型及记录逻辑
- [x] 6.2 在管理后台菜单/导航中增加批量创建入口权限控制

## 7. 测试与验证

- [x] 7.1 编写 `model/user.go::BatchCreateUsers` 的单元测试（事务回滚、用户名冲突场景）
- [ ] 7.2 编写 `model/subscription.go::ActivatePendingSubscriptions` 的单元测试（幂等性、已激活不重复激活、两表同事务更新）
- [ ] 7.3 验证 SQLite / MySQL / PostgreSQL 三种数据库下新表创建和批量创建的正确性
- [ ] 7.4 验证现有 `CreateUser`、`AdminBindSubscription` 等存量接口行为不受影响（不写入新表）
