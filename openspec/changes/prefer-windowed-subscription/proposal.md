## Why

`subscription-daily-time-window` change 引入了"每日可用时段"字段并实现了"窗口外跳过"语义，但其 `Decision 8` 明确遗留了一个债务（`openspec/changes/subscription-daily-time-window/design.md:132-148`）：

> 当用户同时持有"每日窗口订阅"与"全天订阅"且都在窗口内时，系统继续按 `end_time asc, id asc` 排序选第一个候选。这意味着——如果全天订阅的 `end_time` 早于窗口订阅，**窗口订阅永远不会被优先消费**，反而被全天订阅"插队"。

举一个具体痛点：

```
管理员配置：
  A = 夜间套餐 22:00-07:00, end_time=T+30天 (月套餐)
  B = 全天套餐,             end_time=T+7天  (周套餐)

预期心智： 23:00 应该扣 A（夜间套餐优先）
实际行为： 23:00 扣 B（B.end_time 更早，排第一；A 一直没机会被消费，
                     直到 B 用完额度过期）
```

`Decision 8` 当时是有意识的债务——"先发布、看反馈、再决定是否升级"。现在反馈明确：**运营方配置窗口订阅的初衷就是"在该时段优先使用它"**，而不是"在该时段允许它参与排序"。当前排序语义没有把窗口本身作为优先信号，导致运营心智与实际行为错位。

本变更闭合这个债务，仅修正窗口内时段的优先级语义；窗口外行为、钱包降级、`HasActiveUserSubscription` 预检等既有逻辑全部不变。

## What Changes

1. **窗口内时真窗口订阅优先消费**：当用户同时持有"真窗口订阅"（`daily_active_start_minutes != daily_active_end_minutes`）且当前时刻在该订阅窗口内、与其他全天订阅并存时，系统 MUST 优先选中真窗口订阅。仅当所有真窗口订阅都被排除（窗口外 / 额度不足）时，才评估全天订阅。
2. **窗口外 / 全是全天 / 没订阅的场景行为不变**：与 `subscription-daily-time-window` 现状完全一致，不引入新错误码、新字段、新开关。
3. **多真窗口订阅都在窗口内时按 `end_time asc`**：保持现有"早到期先消费"语义，不引入额外的"窗口优先级"概念。
4. **`HasActiveUserSubscription` 与 `UserActiveSubscriptionsAllowWalletOverflow` 不动**：这两个函数的语义是"是否存在可用订阅"和"是否允许走钱包"，与"哪个订阅先扣"正交，本次不变。

非破坏性变更：不引入新数据库字段、不修改 API 契约、不需要 schema migration、不修改前端表单。仅修改 `PreConsumeUserSubscription` 内部候选评估顺序。

## Capabilities

### New Capabilities

无（本变更扩展已存在的 `subscription-daily-window` capability 的内部选择语义）。

### Modified Capabilities

- `subscription-daily-window`：补充一条 requirement 明确"窗口内时真窗口订阅优先于全天订阅被消费"，与现有"扣费时强制校验每日窗口"requirement 协同。

> 注：`openspec/specs/` 当前为空（`subscription-daily-time-window` change 尚未归档），因此本 change 在 `specs/subscription-daily-window/spec.md` 下以 `## ADDED Requirements` 形式追加新 requirement，待两个 change 一同归档时合并。

## Impact

- **后端**：
  - `model/subscription.go:1453-1508`：`PreConsumeUserSubscription` 的候选遍历循环从"单轮串行"改为"两轮扫描"——第一轮只评估"真窗口订阅（在窗口内 + 额度够）"，第二轮退化到现有逻辑（窗口外的真窗口跳过，全天通过）。
  - 不修改 SQL、不修改 schema、不修改其他相关函数（`HasActiveUserSubscription`、`UserActiveSubscriptionsAllowWalletOverflow`、`IsWithinDailyWindow`）。
- **前端**：本次无组件结构变化。可选地在管理端"每日窗口"配置处补充一条 UI 提示文案，告知运营方"在窗口内时段，本订阅会优先于全天订阅被消费"。
- **i18n**：补充 1 条新的提示文案的中英文翻译（如果决定加 UI 提示）。
- **数据库**：无变更。
- **测试**：新增 `PreConsumeUserSubscription` 表驱动测试覆盖 8 个场景（详见 specs）。
- **回归风险**：现有用户如果只持全天订阅或只持单一窗口订阅，行为完全不变（第一轮找不到任何真窗口候选，自然进入第二轮=现状）。
