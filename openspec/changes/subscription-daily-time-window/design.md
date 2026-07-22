## Context

new-api 的订阅套餐（`SubscriptionPlan`）目前只支持"整体生命周期"维度的时间约束——按 year/month/day/hour/custom 计算的 `end_time`。一旦用户订阅被激活（`UserSubscription.status = active`），中继扣费链路就把它视为全天可用。

关键现状：

- 套餐模板在 `model/subscription.go:153` 定义，`UserSubscription` 实例在 `model/subscription.go:260` 定义，扣费链路入口是 `PreConsumeUserSubscription`（`model/subscription.go:1319`），它用 `WHERE user_id = ? AND status = 'active' AND end_time > now` 查候选，按 `end_time asc, id asc` 顺序遍历，逐个判断"额度是否足够"。
- `HasActiveUserSubscription`（`model/subscription.go:1005`）在 token 鉴权时做"是否存在可用订阅"的预检。
- 用户订阅实例已经有"从套餐快照字段"的成熟先例：`UpgradeGroup`、`DowngradeGroup`、`AllowWalletOverflow`、`NextResetTime` 都在创建/激活时从套餐拷贝，避免套餐后续修改影响存量用户。
- 后端约束：所有 SQL 必须兼容 SQLite / MySQL / PostgreSQL，且必须用 GORM 方法；所有 JSON 序列化用 `common.*` 包装；按项目 `AGENTS.md` 不使用 `gorm:"default:true"` 之类的布尔默认值标签。
- 前端：`web/default/src/features/subscriptions/` 是 React 19 + Base UI + Tailwind 的套餐管理模块；i18n 用 `i18next`，英文 key 作为 source。

## Goals / Non-Goals

**Goals:**

1. 在套餐模板与用户订阅实例上引入"每日可用时段"（`daily_active_start_minutes` / `daily_active_end_minutes`），默认值 0/0 等价全天。
2. 在 `PreConsumeUserSubscription` / `HasActiveUserSubscription` 等关键路径强制校验窗口，保持"窗口外跳过→钱包降级"的现有降级语义。
3. 跨午夜窗口（如 23:00–06:00）一等公民化，不需要运营方拆分成两条规则。
4. 管理端能配置与清空窗口；用户端能看到自己订阅的窗口。
5. 100% 向后兼容：升级后既有套餐与既有订阅的行为零变化。

**Non-Goals:**

1. 不做按星期/月日的复合调度（如"仅工作日"）。
2. 不修改订阅整体计费、额度重置、退款、订单回调等既有逻辑。
3. 不为"窗口外"引入新的 HTTP 错误码、新的 `BizError` 或新的前端提示弹窗；继续走"无可用订阅→钱包降级"现有路径。
4. 不引入新的依赖或外部服务（如 cron 调度器）；校验在请求时同步进行。
5. 不修改 `SubscriptionPlanInfo`（窗口校验直接读 `UserSubscription` 快照字段，避免每次扣费多一次 `subscription_plan` 回查）。

## Decisions

### Decision 1: 用"自午夜起的分钟偏移"而非"HH:mm 字符串"存储

**选择**：`int` 分钟偏移（0–1439）。

**理由**：
- 与现有 `CustomSeconds` / `QuotaResetCustomSeconds` 的"整数秒/分钟"风格一致。
- 跨午夜判断只需简单比较，无需解析字符串。
- DB 跨库友好（`type:int;default:0`），无 collation/charset 差异。

**备选**：`varchar(5) "HH:mm"`——被否决，因为每次比较都要解析、容易写错、性能差。

### Decision 2: 用户订阅实例做"快照"，不每次回查套餐

**选择**：在 `UserSubscription` 上冗余两个字段，购买/激活时从套餐拷贝；扣费链路只读 `UserSubscription`。

**理由**：
- 与现有 `UpgradeGroup` / `DowngradeGroup` / `AllowWalletOverflow` 的快照模式完全一致，保持架构统一。
- `PreConsumeUserSubscription` 已经在事务里 `FOR UPDATE` 锁了 `UserSubscription`，不增加额外回查套餐的开销。
- 保护已购用户的契约——运营方修改套餐窗口不会追溯影响存量订阅。

**备选**：每次扣费时 `getSubscriptionPlanByIdTx` 回查套餐并读字段——被否决，因为破坏存量契约、增加事务内的额外读取。

### Decision 3: 窗口外候选"跳过"，不"硬失败"

**选择**：在候选遍历循环里加 `if !IsWithinDailyWindow(...) { continue }`，与"额度不足跳过"同一语义。只有全部候选都不可用时才返回现有的 `subscription quota insufficient` / `no active subscription` 错误。

**理由**：
- 复用现有的"无可用订阅 → 钱包降级"路径，对前端、对 `FundingSource` 选择逻辑零侵入。
- 用户体验一致：窗口外请求会自动回落到钱包扣费，而不是给一个生硬的"现在不能用"错误。
- 避免引入新错误码带来的版本兼容负担。

**备选**：窗口外直接返回新的 `ErrSubscriptionOutOfWindow`——被否决，因为会改变 API 行为契约、需要前端新增提示、且与"额度不足时跳过降级"的语义不一致。

### Decision 4: 共享纯函数 `IsWithinDailyWindow(start, end, now)`

**选择**：在 `model/subscription.go` 提供一个独立的、无副作用的判定函数，被 `PreConsumeUserSubscription`、`HasActiveUserSubscription`、`GetAllActiveUserSubscriptions`、前端展示逻辑（通过 Go API 返回的原始字段在前端独立计算展示，或后端提供 `IsWithinWindow` 字段——见 Decision 6）共享。

**理由**：
- 单点定义跨午夜逻辑，避免散落实现导致的行为漂移。
- 单元测试可表驱动覆盖全天 / 同日 / 跨午夜边界。

**签名草案**：

```go
// IsWithinDailyWindow reports whether the given wall-clock minute-of-day m
// falls inside the daily window defined by [start, end) minutes.
// start == end (incl. 0/0) means "all day".
// start < end means a same-day window.
// start > end means a cross-midnight window.
func IsWithinDailyWindow(start, end, m int) bool {
    if start == end {
        return true
    }
    if start < end {
        return m >= start && m < end
    }
    // cross-midnight
    return m >= start || m < end
}
```

**当前时刻获取**：复用现有的 `GetDBTimestamp()`（基于 `time.Now()`），分钟数用 `time.Now().Local()`（显式 Local，与现有 `time.Date(base.Year(), ...)` 重置逻辑在 `calcNextResetTime` 中的用法一致）。

**Decision 4a（时区）**：所有窗口判定使用服务进程本地时区（`time.Local`）。在 UI 文案与 spec 中明确说明"时间为服务器本地时间"。备选 UTC 被否决，因为与 `calcNextResetTime` 用的本地日界对齐方式一致，且本特性的"日内时段"天然贴近部署地时间。

**Decision 4b（事务内时间一致性）**：`PreConsumeUserSubscription` MUST 在事务开始前缓存一次 `minuteOfDay := currentMinuteOfDay()`，并在 FOR UPDATE 候选循环内复用该值。事务内每轮迭代重新读取系统时间会让不同候选用不同分钟判断，在跨午夜窗口边界附近产生非确定性行为。同样原则已应用于 `HasActiveUserSubscription` 与 `UserActiveSubscriptionsAllowWalletOverflow`。

**Decision 4c（窗口与钱包回退的耦合）**：`UserActiveSubscriptionsAllowWalletOverflow` MUST 把"窗口外"的 strict 订阅视为不可见——即只在用户持有"**当前在窗口内**且 `allow_wallet_overflow=false`"的订阅时才返回 `false`（阻止钱包回退）。否则会出现"用户持有一条窗口外 strict 订阅 → 既不能用订阅（窗口外）也不能用钱包（被 strict 阻止）→ 请求完全被拒"的死锁场景，与"窗口外跳过=视同无订阅→走钱包"的设计直觉相悖。窗口判断在 Go 层做（与 `HasActiveUserSubscription` 一致），因为跨午夜逻辑无法在 SQLite/MySQL/PostgreSQL 三库上用统一 SQL 表达。

### Decision 5: 字段规范化在 model 层做，controller 只做边界拒绝

**选择**：
- `controller/subscription.go` 在 upsert 时校验 `0 <= v <= 1439`，越界返回 400。
- `model.SubscriptionPlan.NormalizeDefaults()` 扩展做"`start == end && start != 0` → 归零"的语义规范化。
- `UserSubscription` 创建/激活路径在快照两个字段前调用同样的规范化。

**理由**：与现有的 `NormalizeResetPeriod` / `NormalizeDefaults` 同一模式，校验与规范化分层清晰。

### Decision 6: 前端展示窗口不发新请求，纯客户端格式化

**选择**：管理端编辑抽屉用 `<input type="time">` 或 Base UI 的 TimePicker（按现有依赖选择）；用户端展示直接用读到的 `daily_active_start_minutes` / `daily_active_end_minutes` 在客户端格式化为 `HH:mm–HH:mm`，不走后端格式化。

**理由**：
- 格式化是无状态纯函数，前端做即可。
- 避免后端 DTO 增加派生字段带来的 schema 复杂度。

### Decision 7: i18n 文案以英文 key 为 source

按项目约定，所有新增 UI 文案以英文 source string 作为 key，在 `en.json` 写英文，`zh.json` 写中文。其余语言（`fr`/`ru`/`ja`/`vi`）的翻译通过 `bun run i18n:sync` 由后续维护补齐，本次变更至少保证 `en` 与 `zh` 完整。

新增 key 草案：
- `Daily Active Window`
- `Daily Active Start Time`
- `Daily Active End Time`
- `Clear (All Day)`
- `This window crosses midnight and will expire at {{end}} the next day.`
- `Daily {{start}}–{{end}} available`
- `Outside this window, other available subscriptions will still be consumed in expiry order.`

### Decision 8: 多订阅共存时保留现有 end_time 排序（已确认方案 1）

当用户同时持有"每日窗口订阅"与"全天订阅"时，系统 MUST 继续按现有的 `end_time asc, id asc` 顺序遍历候选，对每条候选执行"窗口 + 额度"双重过滤，选中第一条通过的候选。

含义：

- 不引入"窗口外跨过其他全天订阅直接回落钱包"的隐式优先级。
- 不引入"优先级"字段。
- 若每日窗口订阅的 end_time 早于全天订阅：窗口外时段会评估并扣全天订阅（用户的月度/年度订阅会被消耗）。
- 若全天订阅的 end_time 早于每日窗口订阅：全天订阅先被消耗完，每日窗口订阅在其后才生效。
- 若用户只持有每日窗口订阅、无其他可用订阅，且在窗口外：回落到钱包扣费（与 Decision 3 一致）。

**理由**：零侵入；与现有"按到期顺序消费"语义一致；避免引入难以向运营方解释的隐式优先级。

**已确认**：propose 阶段用户明确选择方案 1。后续如运营方需要"窗口外保护其他订阅不被消耗"，再开新变更评估方案 2（窗口外跳过所有候选直接回落钱包）或方案 3（显式优先级字段）。

UI 提示责任：管理端配置每日窗口时 MUST 显示提示 `Outside this window, other available subscriptions will still be consumed in expiry order.`，让运营方知晓此共存语义。

## Risks / Trade-offs

- **[时区漂移]** 服务器迁移部署时区会导致窗口边界跳变。→ **缓解**：spec 与 UI 文案明确"服务器本地时间"；未来可在系统设置里加时区开关，但本次不做。
- **[用户困惑]** 窗口外请求自动扣钱包，用户可能不理解"为什么这次没用套餐"。→ **缓解**：用户端订阅卡片显式展示窗口；日志中已有 `billing_source` 字段可追溯。
- **[跨午夜边界误判]** `start == end`（如 600/600）按"全天"处理可能让运营方困惑。→ **缓解**：在 model 规范化时把 `start == end && start != 0` 归零；管理端 UI 在两值相等时提示"等同全天，已自动归零"。
- **[存量数据兼容性]** 既有 `UserSubscription` 的新列默认 0，符合"全天可用"语义，无需数据回填。→ **无需额外迁移脚本**，GORM `AutoMigrate` 足矣。
- **[AutoMigrate 重复 ALTER]** 按项目 `AGENTS.md` 警示，避免在 boolean default 上重复 ALTER——本次新增字段是 `int default:0`，三个数据库行为稳定，无此风险。
- **[扣费事务耗时]** 窗口校验是纯内存整数比较，对事务耗时无可观测影响。
- **[并发与缓存]** `subscriptionPlanCache` 缓存的是整个 `SubscriptionPlan`，新增字段自动随缓存；不需要调整 cache key/version（但建议在 `subscriptionPlanCacheNamespace` 之外通过字段本身的语义保证向后兼容——本次默认值 0 已保证）。

## Migration Plan

1. **代码变更**（单一 PR）：
   - `model/subscription.go`：两个结构体新增字段、`IsWithinDailyWindow`、`NormalizeDefaults` 扩展、`PreConsumeUserSubscription` / `HasActiveUserSubscription` / `GetAllActiveUserSubscriptions` 增加窗口校验、`UserSubscription` 创建路径增加快照。
   - `controller/subscription.go`：upsert 校验 0–1439。
   - `web/default/src/features/subscriptions/`：types/lib/drawer/constants 同步。
   - `web/default/src/i18n/locales/en.json` / `zh.json`：新增 key。
   - 单元测试：`IsWithinDailyWindow` 表驱动测试、`PreConsumeUserSubscription` 窗口外跳过的表驱动测试、`NormalizeDefaults` 等值归零测试。

2. **部署顺序**：
   - 服务启动时 GORM `AutoMigrate` 自动添加 4 列（默认 0）。
   - 升级后，既有套餐与既有订阅继续表现为全天可用——零行为变化。
   - 运营方在前端逐步给指定套餐配置窗口；新购用户立即受窗口约束；存量用户保持快照值 0/0 不受影响。

3. **回滚策略**：
   - 单纯代码回滚即可——新列保留也无害（默认 0，等价全天）。
   - 如需彻底清理，可手动 `ALTER TABLE ... DROP COLUMN`（PostgreSQL/MySQL 支持；SQLite 需重建表），但**不推荐**也不属于本次回滚路径。

## Open Questions

1. **是否需要在管理端提供"按当前服务器时间预览当前是否可用"的指示灯？** 当前设计不包含；如运营方反馈需要，可在后续 PR 加一个 `IsWithinDailyWindow` 客户端预览。
2. **是否需要在管理端批量配置多个套餐的窗口？** 当前设计只支持单套餐编辑，沿用现有抽屉交互；批量需求如出现可在后续迭代加。
3. **用户端是否需要在请求被窗口拦截时给出站内提示？** 当前完全静默降级到钱包，符合本次"不引入新提示"的非目标；待用户反馈后再评估。
