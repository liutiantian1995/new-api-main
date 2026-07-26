## Context

`subscription-daily-time-window` 已经在 `model/subscription.go` 引入了：

- `UserSubscription.DailyActiveStartMinutes` / `DailyActiveEndMinutes`（快照字段，默认 0/0 表全天）
- `IsWithinDailyWindow(start, end, m)` 纯函数（line 246-260），处理同日/跨午夜
- `PreConsumeUserSubscription`（line 1401-1514）的候选遍历循环，结构如下：

```go
// model/subscription.go:1443-1508
var subs []UserSubscription
DB.Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
   Order("end_time asc, id asc").Find(&subs)

minuteOfDay := currentMinuteOfDay()
for _, candidate := range subs {
    sub := candidate
    // ... maybeReset ...
    if !IsWithinDailyWindow(sub.DailyActiveStartMinutes, sub.DailyActiveEndMinutes, minuteOfDay) {
        continue
    }
    if sub.AmountTotal > 0 && (sub.AmountTotal - sub.UsedBefore) < amount {
        continue
    }
    // 选中，扣费 + return
}
return errors.New("subscription quota insufficient, ...")
```

关键约束（来自 `subscription-daily-time-window/design.md`）：

- **Decision 4c**：跨午夜窗口判断必须在 Go 层做（无法在 SQLite/MySQL/PostgreSQL 三库用统一 SQL 表达）。
- **Decision 8**：当时明确选择"保留 `end_time asc, id asc` 排序"，并预留方案 2（窗口外跳过所有候选）和方案 3（优先级字段）作为后续债务。
- **Decision 4b**：`minuteOfDay` 在事务开始前缓存一次，整个 `FOR UPDATE` 循环复用，避免跨分钟边界漂移。

约束：本次变更**必须保留 Decision 4b 的时间一致性**与**Decision 4c 的 Go 层判断**，不能在 SQL 排序里加 case when。

## Goals / Non-Goals

**Goals:**

1. 让"配置了每日窗口的订阅"在窗口内时段**优先于全天订阅**被消费，符合运营方配置窗口订阅的心智模型。
2. 与现有 `daily_active_window` 字段、`IsWithinDailyWindow` 函数、`HasActiveUserSubscription` / `UserActiveSubscriptionsAllowWalletOverflow` 等周边函数零侵入协同。
3. 100% 向后兼容：只持全天订阅 / 只持单一窗口订阅 / 多个全天订阅并存的用户，行为完全不变。
4. 不引入新数据库字段、新 API 错误码、新前端表单字段、新系统开关。

**Non-Goals:**

1. **不解决"窗口外保护全天订阅不被消耗"**：当前债务的另一面（窗口外不要扣全天订阅、直接走钱包）。如运营方反馈需要，再开新变更评估方案 2。
2. **不引入显式 priority 字段**：方案 3 被明确否决——UI 复杂度与运营心智成本高于收益。
3. **不修改 `HasActiveUserSubscription` 与 `UserActiveSubscriptionsAllowWalletOverflow`**：这两个函数语义是"是否有可用订阅"与"是否允许走钱包"，与"哪个先扣"正交。
4. **不引入多窗口订阅间的优先级排序**：两个真窗口订阅都在窗口内时，仍按 `end_time asc` 评估，避免引入"窗口优先级"概念。
5. **不修改 SQL 候选查询**：跨午夜判断在 Go 层做（Decision 4c 约束），不能在 `ORDER BY` 加窗口优先级。

## Decisions

### Decision 1: 用"两轮扫描"而非"排序重排"

**选择**：把 `PreConsumeUserSubscription` 的候选遍历循环从单轮改成两轮：

```
第一轮（优先候选）：
  for sub in subs:
    if not isRealWindow(sub): continue        ← 全天订阅跳过
    if not IsWithinDailyWindow(sub, now): continue  ← 真窗口但不在窗口内跳过
    if remain < amount: continue              ← 额度不足跳过
    扣 sub, return

第二轮（常规候选，等价于现状）：
  for sub in subs:
    if isRealWindow(sub) and not IsWithinDailyWindow(sub, now):
      continue                                 ← 真窗口且不在窗口内，跳过
    if not IsWithinDailyWindow(sub, now): continue  ← 实际只对全天生效（永远 true）
    if remain < amount: continue
    扣 sub, return

return "subscription quota insufficient, ..."
```

其中 `isRealWindow(sub) = sub.DailyActiveStartMinutes != sub.DailyActiveEndMinutes`。

**备选方案 A：Go 层 `sort.SliceStable`** —— 被否决，因为：
1. 两轮扫描语义更清晰（第一轮=优先，第二轮=常规），代码可读性更高
2. 与现有 `for-range-continue` 结构改动最小
3. 不需要额外分配排序数组（`subs` 来自 GORM 查询，直接复用）

**备选方案 B：SQL `ORDER BY` 加窗口优先级** —— 被否决，因为 Decision 4c 明确禁止（跨午夜逻辑无法在 SQLite/MySQL/PostgreSQL 三库用统一 SQL 表达）。

**为什么两轮扫描是正确的**：

- 第一轮的"找不到候选"不报错，自然进入第二轮——保证向后兼容（只持全天订阅的用户，第一轮全 continue，第二轮选中全天，行为=现状）
- 第二轮与现有循环结构**完全等价**（全天订阅的 `IsWithinDailyWindow` 永远返回 true；真窗口订阅的窗口外检查在两轮里都生效）
- "额度不够自动 fallback 到下一个"语义保留（每个候选独立 `continue`）

### Decision 2: `isRealWindow` 判定基于"start != end"

**选择**：`isRealWindow(sub) = sub.DailyActiveStartMinutes != sub.DailyActiveEndMinutes`。

**理由**：
- 与 `subscription-daily-time-window` 的字段规范化规则一致——`SubscriptionPlan.NormalizeDefaults()`（`model/subscription.go`）已经把"`start == end`（含非零）→ 归零"，因此数据库里的真窗口订阅必然满足 `start != end`。
- 不需要新字段、不需要额外元信息。

**边界情况**：如果未来引入"start/end 同值但不归零"的语义（极不可能），`isRealWindow` 需要同步调整。本次不考虑。

### Decision 3: 多真窗口订阅并存时保持 `end_time asc`

**选择**：当多个真窗口订阅同时都在窗口内时（例如夜间 22-07 套餐 A 与工作日 09-18 套餐 B，当前 23:00 只有 A 在窗口；但若配置允许两个窗口重叠时段），第一轮按现有 `end_time asc, id asc` 顺序评估，不引入"窗口越窄越优先"等额外规则。

**理由**：
- 避免引入"窗口优先级"概念，与 Decision 8 的"不引入显式 priority 字段"精神一致。
- 真窗口订阅并存且同时段重叠是边界场景，运营方通常会错峰配置；过度设计收益低。
- 如运营方反馈需要，可后续评估方案 3（priority 字段）。

### Decision 4: `HasActiveUserSubscription` 与 `UserActiveSubscriptionsAllowWalletOverflow` 不修改

**选择**：保持现状。

**为什么 `HasActiveUserSubscription` 不需要改**：
- 它的语义是"是否存在可用订阅"（用于 token 鉴权预检）
- "可用"已经包含窗口过滤（Decision 4c）
- 修改它会让"有窗口订阅但窗口外"的用户在 token 鉴权时被拒绝——违背"窗口外跳过→钱包降级"的整体设计

**为什么 `UserActiveSubscriptionsAllowWalletOverflow` 不需要改**：
- 它的语义是"是否允许走钱包"——只在用户持有 `allow_wallet_overflow=false` 且"当前可见"的订阅时返回 `false`
- 与"哪个订阅先扣"正交，不影响钱包回退决策

### Decision 5: 可选的 UI 提示文案

**选择**：在管理端"每日窗口"配置处补充一条提示：

> `Inside this window, this subscription is consumed before any all-day subscriptions.`

中文：
> `在窗口内时段，本订阅会优先于全天订阅被消费。`

这是**可选的**——不加也不会影响功能正确性，但加了能减少运营方对"为什么我的全天套餐突然不扣了"的困惑。

## Risks / Trade-offs

- **[真窗口订阅被消耗更快]** 用户感知上"夜间套餐扣得快了"——但这正是符合预期的（之前可能一直扣全天套餐）。→ **缓解**：可选 UI 提示；日志中已有 `billing_source` 字段可追溯。
- **[多真窗口订阅都窗口内的边界]** 当前实现按 `end_time asc`，可能不符合某些运营方的"窄窗口优先"心智。→ **缓解**：本次明确接受这个边界，留给后续变更。
- **[测试覆盖]** 必须覆盖 8 个核心场景（详见 specs）。→ **缓解**：表驱动测试 + 显式断言。
- **[`HasActiveUserSubscription` 一致性担忧]** 表面上"窗口内时优先扣窗口订阅"似乎应该影响"是否有可用订阅"的判断，但实际不影响——因为该函数只判断"是否存在"，不判断"哪个先扣"。→ **无需缓解**，已在 Decision 4 说明。
- **[存量订阅兼容性]** 不引入新字段，老订阅的 `start==end==0` 自动归为"非真窗口"，第一轮全跳过，第二轮选中——行为=现状。→ **无需迁移**。

## Migration Plan

1. **代码变更**（单一 PR）：
   - `model/subscription.go:1453-1508`：拆成两轮循环；提取 `isRealWindow` 内联函数或局部变量。
   - 新增表驱动测试覆盖 8 个场景。
   - 可选：`web/default/src/features/subscriptions/...` 补充 UI 提示文案 + i18n。
2. **部署**：无 schema 变更，无配置变更，无开关。部署后立即生效。
3. **回滚**：纯代码回滚即可。

## Open Questions

1. **是否需要在用户端订阅卡片也展示优先消费提示？** 当前设计不包含；如用户反馈"为什么我的全天套餐不扣了"，可后续在用户端加一条"本订阅在窗口内时段优先消费"提示。
2. **是否需要管理员端"按当前服务器时间预览哪个订阅会被扣"的指示灯？** 已在 `subscription-daily-time-window/design.md` 的 Open Question 1 提过，本次不引入。
3. **日志是否需要记录"为什么选中了这个订阅"的 reason？** 当前 `billing_source` 字段只记录订阅 ID 与金额，不记录选择原因。如需要，可后续在 `BillingSession` 加 `selection_reason` 字段。
