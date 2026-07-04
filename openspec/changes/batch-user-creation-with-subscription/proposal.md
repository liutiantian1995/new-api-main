## Why

管理员目前只能逐个创建用户，无法批量创建。在需要一次性创建大量用户（如企业客户、活动发放账号）时，效率极低。同时，现有订阅管理是即时生效的，对于"按月/按天"计费的套餐，用户创建后即使未使用也会开始计时，不够合理。需要提供批量创建用户的能力，并支持订阅的延迟生效策略。

## What Changes

1. **新增批量创建用户 API**：管理员可通过前缀+日期后缀批量创建用户，自动生成密码（用户名+@123），支持批量设置分组、批量关联套餐订阅、批量创建 API 密钥。
2. **新增订阅生效策略**：支持两种生效模式：
   - `immediate`：立即生效（现有行为，订阅创建后立即开始计时）
   - `on_use`：使用时生效（用户首次登录或首次使用 API 密钥时，订阅才开始计时）
3. **新增"使用时生效"调度逻辑**：在用户登录验证或令牌验证时任一触发即检查并激活待生效的订阅（两者是"或"的关系，不需要同时发生）。
4. **新增批量用户创建前端页面**：在管理后台用户管理区域增加批量创建入口和表单。

## Capabilities

### New Capabilities

- `batch-user-creation`：批量创建用户，支持前缀命名规则、默认密码策略、批量分组设置、批量订阅关联、批量密钥创建，以及订阅生效策略配置。
- `subscription-activation-strategy`：订阅生效策略（immediate/on_use），在使用时生效模式下延迟订阅计时起点至用户首次使用。

### Modified Capabilities

- 无现有 capability 的需求变更，所有新增功能在现有接口上扩展。

## Impact

- **后端**：新增 `controller/user.go` 中的 `BatchCreateUsers` 处理函数；新增 `model/user.go` 中的 `BatchCreateUsers` 方法；新增 `model/subscription.go` 中的 `AdminBindSubscriptionWithStrategy` 方法和 `ActivatePendingSubscriptions` 方法；扩展存量 `AdminCreateUserSubscription` 控制器和 `AdminBindSubscription` 方法支持可选 `activation_strategy` 参数；修改 `controller/user.go` 中的 `setupLogin` 流程以触发订阅激活。
- **前端**：新增批量创建用户页面组件（`web/default/src/features/users/components/batch-create-drawer.tsx`）；修改用户管理页面增加批量创建入口按钮；在"用户订阅管理"弹窗中新增"生效策略"下拉选项。
- **数据库**：新增 `pending_subscription_activations` 表（不修改现有 `user_subscriptions` 表），通过 GORM AutoMigrate 创建。
- **i18n**：新增批量创建相关的前后端翻译 key。
