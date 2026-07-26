## ADDED Requirements

### Requirement: 窗口内时真窗口订阅优先于全天订阅被消费

当 `PreConsumeUserSubscription` 评估候选订阅时，MUST 按"两轮扫描"顺序选择：

1. **第一轮（优先候选）**：仅评估"真窗口订阅"——即 `daily_active_start_minutes != daily_active_end_minutes` 的订阅。对每个真窗口订阅执行"当前时刻在窗口内 + 剩余额度足够"的双重过滤，第一个通过的候选 MUST 被选中并扣费。
2. **第二轮（常规候选）**：若第一轮没有候选通过，MUST 退化到现有评估顺序——对所有候选（含全天订阅与窗口外的真窗口订阅）按 `end_time asc, id asc` 排序，逐个执行"窗口过滤 + 额度过滤"，第一个通过的候选 MUST 被选中并扣费。

"真窗口订阅"判定规则：`daily_active_start_minutes != daily_active_end_minutes`（与 `SubscriptionPlan.NormalizeDefaults` 的"`start == end` 归零"规则协同，保证数据库中的真窗口订阅必然满足该条件）。

当且仅当两轮扫描都没有候选通过时，MUST 返回现有的"无可用订阅"错误（不引入新错误码），上层按现有路径回落到钱包或返回标准错误。

此 requirement 与 `subscription-daily-time-window` 已有的"扣费时强制校验每日窗口"requirement 协同——后者定义"窗口外跳过"，本 requirement 定义"窗口内优先"。

#### Scenario: 23:00 真窗口订阅 + 全天订阅，全天订阅 end_time 更早

- **WHEN** 用户持有订阅 A（窗口 22:00–07:00，`daily_active_start_minutes=1320, daily_active_end_minutes=420`，`end_time=T+30天`）
- **AND** 用户持有订阅 B（全天，`daily_active_start_minutes=0, daily_active_end_minutes=0`，`end_time=T+7天`）
- **AND** 当前本地时间为 23:00（minuteOfDay=1380）
- **THEN** 第一轮 MUST 跳过 B（非真窗口），评估 A 并通过（1380 >= 1320 为真，且窗口内）
- **AND** `PreConsumeUserSubscription` MUST 选中 A 并扣 A 的额度
- **AND** MUST NOT 评估或扣费 B

#### Scenario: 23:00 真窗口订阅 + 全天订阅，真窗口订阅 end_time 更早

- **WHEN** 用户持有订阅 A（窗口 22:00–07:00，`end_time=T+7天`）
- **AND** 用户持有订阅 B（全天，`end_time=T+30天`）
- **AND** 当前本地时间为 23:00
- **THEN** 第一轮 MUST 评估 A（真窗口，在窗口内）并通过
- **AND** MUST 选中 A 并扣 A 的额度
- **AND** 行为与本次变更前一致（因为 A 在 SQL 排序中也排第一）

#### Scenario: 10:00 真窗口订阅 + 全天订阅（窗口外）

- **WHEN** 用户持有订阅 A（窗口 22:00–07:00）和订阅 B（全天）
- **AND** 当前本地时间为 10:00（minuteOfDay=600）
- **THEN** 第一轮 MUST 评估 A（真窗口）但 `IsWithinDailyWindow` 返回 false（600 >= 1320 与 600 < 420 均为 false）→ continue
- **AND** 第一轮结束后没找到候选，进入第二轮
- **AND** 第二轮 MUST 跳过 A（真窗口且窗口外），评估 B（全天，窗口判定永远 true）并通过
- **AND** MUST 选中 B 并扣 B 的额度
- **AND** 行为与本次变更前一致

#### Scenario: 真窗口订阅窗口内但额度不足时自动 fallback 到全天订阅

- **WHEN** 用户持有订阅 A（窗口 22:00–07:00，`amount_total=100`，`amount_used=100`，已耗尽）和订阅 B（全天，额度充足）
- **AND** 当前本地时间为 23:00（在 A 的窗口内）
- **THEN** 第一轮 MUST 评估 A：窗口判定通过，但 `remain=0 < amount` → continue
- **AND** 第一轮结束后没找到候选，进入第二轮
- **AND** 第二轮 MUST 跳过 A（真窗口且窗口外判定为 false 不成立，但 A 已 continue 是因为额度——实际上 A 在第二轮也会因额度不足 continue）
- **AND** 第二轮 MUST 评估 B 并通过
- **AND** MUST 选中 B 并扣 B 的额度

#### Scenario: 多个真窗口订阅都在窗口内时按 end_time asc

- **WHEN** 用户持有订阅 A（窗口 22:00–07:00，`end_time=T+7天`）
- **AND** 用户持有订阅 C（窗口 20:00–09:00，`end_time=T+30天`）
- **AND** 当前本地时间为 23:00（同时在 A 与 C 的窗口内）
- **THEN** 第一轮 MUST 按 SQL 返回的 `end_time asc, id asc` 顺序遍历
- **AND** A.end_time 早于 C.end_time → A 排在 C 前面
- **AND** MUST 评估 A（真窗口，在窗口内，假设额度足够）并通过
- **AND** MUST 选中 A 并扣 A 的额度
- **AND** MUST NOT 引入"窗口越窄越优先"等额外规则

#### Scenario: 只持有全天订阅时行为完全不变

- **WHEN** 用户仅持有订阅 B（全天，额度充足）
- **AND** 当前本地时间为任意时刻
- **THEN** 第一轮 MUST 跳过 B（非真窗口），第一轮结束无候选
- **AND** 第二轮 MUST 评估 B（窗口判定永远 true）并通过
- **AND** MUST 选中 B 并扣 B 的额度
- **AND** 行为与本次变更前完全一致

#### Scenario: 只持有真窗口订阅且在窗口外时回落钱包

- **WHEN** 用户仅持有订阅 A（窗口 22:00–07:00，额度充足）
- **AND** 当前本地时间为 10:00（在 A 的窗口外）
- **THEN** 第一轮 MUST 评估 A 但窗口判定为 false → continue
- **AND** 第一轮结束无候选
- **AND** 第二轮 MUST 跳过 A（真窗口且窗口外）
- **AND** 第二轮结束无候选
- **AND** `PreConsumeUserSubscription` MUST 返回"无可用订阅"错误
- **AND** 上层（`FundingSource` 选择链路）MUST 按现有路径回落到钱包余额扣费

#### Scenario: 真窗口订阅 + 全天订阅都在第一轮被排除时退化到第二轮

- **WHEN** 用户持有订阅 A（窗口 22:00–07:00，已耗尽额度）和订阅 B（全天，已耗尽额度）
- **AND** 当前本地时间为 23:00
- **THEN** 第一轮 MUST 评估 A：窗口通过但额度不足 → continue
- **AND** 第一轮结束无候选
- **AND** 第二轮 MUST 评估 A：再次因额度不足 continue；评估 B：因额度不足 continue
- **AND** 第二轮结束无候选
- **AND** `PreConsumeUserSubscription` MUST 返回"subscription quota insufficient"错误
- **AND** 上层按现有路径处理（钱包降级或返回标准错误）

---

### Requirement: 两轮扫描的实现约束

`PreConsumeUserSubscription` 的两轮扫描实现 MUST 满足以下工程约束：

1. **共享 `minuteOfDay` 快照**：两轮扫描 MUST 复用同一个在事务开始前缓存的 `minuteOfDay` 值，与 `subscription-daily-time-window` Decision 4b 的时间一致性原则协同，避免跨分钟边界漂移。
2. **共享 `subs` 候选列表**：两轮 MUST 遍历同一个 SQL 查询返回的 `subs` 切片，不重新查询数据库。
3. **第二轮等价于现状**：第二轮的循环逻辑 MUST 与本次变更前的 `PreConsumeUserSubscription` 完全等价——对全天订阅永远通过窗口判定，对真窗口订阅保留窗口外 continue。
4. **不修改 SQL**：候选 SQL 查询 MUST 保持 `WHERE user_id = ? AND status = 'active' AND end_time > ?` + `ORDER BY end_time asc, id asc`，不引入窗口优先级排序。
5. **不修改其他相关函数**：`HasActiveUserSubscription`、`UserActiveSubscriptionsAllowWalletOverflow`、`IsWithinDailyWindow`、`SubscriptionPlan.NormalizeDefaults` MUST 保持现状。

#### Scenario: minuteOfDay 在两轮中保持一致

- **WHEN** `PreConsumeUserSubscription` 进入第一轮循环前缓存了 `minuteOfDay = 1380`
- **AND** 第一轮循环耗时跨越了分钟边界（系统时间从 22:59:59 跳到 23:00:00）
- **THEN** 第一轮所有候选 MUST 用 `minuteOfDay = 1380` 判定
- **AND** 第二轮所有候选 MUST 用同一个 `minuteOfDay = 1380` 判定
- **AND** MUST NOT 在第二轮重新调用 `currentMinuteOfDay()`

#### Scenario: 第二轮等价于本次变更前的行为

- **WHEN** 任意用户场景下进入第二轮
- **THEN** 第二轮选中的候选 MUST 与本次变更前 `PreConsumeUserSubscription` 选中的候选完全一致
- **AND** 此不变量可通过单测验证（构造相同输入，断言两轮实现与旧实现选中相同订阅）

#### Scenario: SQL 查询保持不变

- **WHEN** 任意用户请求触发 `PreConsumeUserSubscription`
- **THEN** 后端日志（如果开启 SQL trace）MUST 显示候选查询仍然是 `SELECT ... FROM user_subscriptions WHERE user_id = ? AND status = 'active' AND end_time > ? ORDER BY end_time asc, id asc`
- **AND** MUST NOT 出现 `ORDER BY (CASE WHEN ...)` 或类似的窗口优先级 SQL

---

### Requirement: 真窗口订阅判定基于 start != end

`isRealWindow(sub)` 判定规则 MUST 为 `sub.DailyActiveStartMinutes != sub.DailyActiveEndMinutes`。

此判定与 `SubscriptionPlan.NormalizeDefaults` 的"`start == end`（含非零）→ 归零"规则协同，保证数据库中：

- `0/0` → 全天订阅（非真窗口）
- `start != end` → 真窗口订阅
- 不存在"`start == end` 且非 0"的脏数据（被 NormalizeDefaults 归零）

#### Scenario: 全天订阅被识别为非真窗口

- **WHEN** 订阅 S 的 `daily_active_start_minutes=0` 且 `daily_active_end_minutes=0`
- **THEN** `isRealWindow(S)` MUST 返回 `false`
- **AND** S 在第一轮扫描中 MUST 被跳过

#### Scenario: 真窗口订阅被识别为真窗口

- **WHEN** 订阅 S 的 `daily_active_start_minutes=1320` 且 `daily_active_end_minutes=420`
- **THEN** `isRealWindow(S)` MUST 返回 `true`
- **AND** S 在第一轮扫描中 MUST 被评估（前提是当前时刻在窗口内且额度充足）

#### Scenario: 不存在 start == end 且非 0 的脏数据

- **WHEN** 任意路径尝试写入 `daily_active_start_minutes=600, daily_active_end_minutes=600`
- **THEN** `SubscriptionPlan.NormalizeDefaults` MUST 在写入前归零为 `0/0`
- **AND** 数据库中 MUST NOT 出现 `start == end` 且非 0 的记录
- **AND** `isRealWindow` 判定结果与"全天订阅"一致（返回 false）
