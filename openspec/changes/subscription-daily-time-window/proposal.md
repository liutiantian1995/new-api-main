## Why

当前订阅套餐只在整体生命周期层面受限（按年/月/日/小时计的 `end_time`），一旦激活就在有效期内全天 24 小时可用。运营方缺少一种"按日内时段"控制套餐可用性的手段——例如希望把某个低价套餐限制在闲时（23:00–06:00）使用，以错峰引流、控制成本或满足合规要求。本次变更新增"每日可用时段"能力，让套餐级别支持 `生效时间 / 失效时间` 配置，并在 API 中继扣费时强制校验，使订阅的使用时间在日内也可被合理管控。

## What Changes

- 在 `SubscriptionPlan`（套餐模板）中新增每日可用时段字段：`daily_active_start_minutes` / `daily_active_end_minutes`（自午夜 0:00 起的分钟偏移，0–1439；`end < start` 表示跨天窗口，如 23:00→06:00）。两者都为 0 时表示无日内限制（与现有行为一致）。
- 在 `UserSubscription`（用户订阅实例）中快照上述两个字段，确保套餐后续修改不影响已购用户的契约（与 `UpgradeGroup` / `DowngradeGroup` / `AllowWalletOverflow` 的快照模式一致）。
- 在订阅扣费链路（`PreConsumeUserSubscription` 选号 + `HasActiveUserSubscription` 预检）中加入"当前时间是否落在套餐每日窗口内"的校验：
  - 不在窗口内时，跳过该订阅（不报错给钱包降级路径），与"额度不足时跳过"保持一致的降级语义；
  - 若所有候选订阅都被日内窗口排除，则按现有的"无可用订阅"路径回落到钱包或返回标准错误（不引入新的错误码体系）。
- 在管理端订阅套餐编辑抽屉（`SubscriptionsMutateDrawer`）中新增"每日可用时段"两个时间选择器（HH:mm），支持跨天窗口（提示文案说明），并提供清空按钮（=全天）。
- 前端类型 `subscriptionPlanSchema` / `userSubscriptionSchema` 增加对应字段并做范围校验（0–1439）。
- 后端校验在创建/更新套餐时执行（normalize 到 0–1439；`start == end && != 0` 视为全天，等价 0/0）。
- 为新增字段补充 i18n key（zh/en 为基准；其余语言沿用英文 key 模式，由 `bun run i18n:sync` 维护）。
- 数据库迁移：通过 GORM `AutoMigrate` 自动添加新列，兼容 SQLite/MySQL/PostgreSQL（默认 0，向后兼容）。

非目标（明确不做）：
- 不引入按星期/按月日的中短期调度（本次只做"每日时段"）。
- 不改变订阅整体计费逻辑（仍是 `start_time` / `end_time` + 额度重置周期）。
- 不引入新的错误码或新的 HTTP 状态（继续复用现有"订阅不可用→钱包降级"路径）。

## Capabilities

### New Capabilities
- `subscription-daily-window`: 订阅套餐的每日可用时段控制——在套餐模板与用户订阅实例上定义日内生效/失效时间，并在中继扣费时强制校验。

### Modified Capabilities
<!-- 无既有 specs，全部以新能力形式引入。 -->

## Impact

- **后端模型**：`model/subscription.go`（`SubscriptionPlan`、`UserSubscription` 两个结构体新增字段；`Normalize*` / 创建快照逻辑更新）。
- **后端扣费链路**：`model/subscription.go` 的 `PreConsumeUserSubscription`、`HasActiveUserSubscription`、`GetAllActiveUserSubscriptions` 中加入日内窗口校验；抽取共享 helper（如 `IsWithinDailyWindow(start, end, now time.Time) bool`）。
- **后端校验**：`controller/subscription.go` 在 upsert 套餐时对 `daily_active_*_minutes` 做 0–1439 normalize。
- **数据库**：GORM `AutoMigrate` 添加 4 列（计划表 2 列 + 用户订阅表 2 列），跨库兼容；现有行默认 0（=全天，零行为变化）。
- **前端管理端**：`web/default/src/features/subscriptions/`（types.ts schema、`lib.ts` form schema 与转换函数、`subscriptions-mutate-drawer.tsx` 表单 UI、`constants.ts` 选项）。
- **前端用户端**：`user-subscriptions-dialog.tsx` 在订阅列表中展示每日窗口（仅展示，便于用户知晓）。
- **i18n**：`web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json` 新增 key。
- **缓存**：订阅计划缓存 (`subscriptionPlanCache`) 不变（字段随计划一起缓存）；`SubscriptionPlanInfo` 暂不扩展（窗口校验在 `UserSubscription` 快照字段上完成，避免每次扣费都回查计划）。
- **测试**：后端为 `IsWithinDailyWindow` 与跨天窗口补单元测试；为 `PreConsumeUserSubscription` 在窗口外被跳过的行为补表驱动测试。
- **兼容性**：默认值 0/0 等价于"全天可用"，现有套餐与存量用户订阅升级后行为不变。
