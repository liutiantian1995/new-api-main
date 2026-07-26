## 1. 后端：PreConsumeUserSubscription 改造为两轮扫描

- [x] 1.1 在 `model/subscription.go` 的 `PreConsumeUserSubscription` 函数内（当前 line 1453-1508 的单轮循环），重构为两轮扫描结构：
  - 提取内联判定 `isRealWindow := func(sub UserSubscription) bool { return sub.DailyActiveStartMinutes != sub.DailyActiveEndMinutes }`（或直接在循环条件里写）
  - 第一轮：`for _, candidate := range subs { if !isRealWindow(candidate) { continue }; if !IsWithinDailyWindow(...) { continue }; if 额度不足 { continue }; 扣费 + return }`
  - 第二轮：`for _, candidate := range subs { if isRealWindow(candidate) && !IsWithinDailyWindow(...) { continue }; if !IsWithinDailyWindow(...) { continue }; if 额度不足 { continue }; 扣费 + return }`
  - 共享同一个 `minuteOfDay := currentMinuteOfDay()` 快照（已有，不重复调用）
  - 两轮遍历同一个 `subs` 切片，不重新查询数据库
- [x] 1.2 验证第二轮等价于现状：手工对比改动前后的第二轮循环逻辑，确认对全天订阅永远通过窗口判定、对真窗口订阅保留窗口外 continue

## 2. 单元测试：覆盖 8 个核心场景

- [x] 2.1 在 `model/subscription_test.go`（或新建 `model/subscription_prefer_window_test.go`）新增表驱动测试 `TestPreConsumeUserSubscription_WindowedPriority`，覆盖以下场景：
  - 场景 1：23:00 真窗口 + 全天，全天 end_time 更早 → 扣真窗口
  - 场景 2：23:00 真窗口 + 全天，真窗口 end_time 更早 → 扣真窗口（与现状一致）
  - 场景 3：10:00 真窗口 + 全天（窗口外）→ 扣全天
  - 场景 4：真窗口在窗口内但额度不足 → fallback 到全天
  - 场景 5：多个真窗口订阅都在窗口内 → 按 end_time asc
  - 场景 6：只持全天订阅 → 无变化
  - 场景 7：只持真窗口订阅且窗口外 → 返回"无可用订阅"错误
  - 场景 8：所有订阅额度都耗尽 → 返回"subscription quota insufficient"错误
- [x] 2.2 每个场景显式断言：选中的订阅 ID、扣费的订阅 ID、错误返回值（如有）
- [x] 2.3 测试 MUST 使用 `testify/require` 做 fatal 断言，`testify/assert` 做非致命检查（按项目 `AGENTS.md` 规则）

## 3. 前端 UI 提示文案（可选）

- [x] 3.1 在 `web/default/src/features/subscriptions/...`（套餐编辑抽屉的"每日窗口"配置区域附近）补充一条 `FormDescription` 文案：英文 `Inside this window, this subscription is consumed before any all-day subscriptions.`，中文 `在窗口内时段，本订阅会优先于全天订阅被消费。`
- [x] 3.2 在 `web/default/src/i18n/locales/en.json` 与 `zh.json` 新增上述 key 的翻译

## 4. 回归验证

- [x] 4.1 运行 `go build ./...` 与 `go test ./model/... -run TestPreConsumeUserSubscription`，确保编译通过且新测试全部通过
- [x] 4.2 运行 `bun run tsc --noEmit`（在 `web/default/`），确保前端类型检查通过（仅当任务 3 已完成时）
- [ ] 4.3 手动回归（启动后端 + 前端，使用 root 账号）：
  - 给测试用户绑定 A（夜间 22-07 月套餐）+ B（全天周套餐）
  - 在 23:00 时段发起请求，验证扣 A 而非 B（可通过数据库 `user_subscriptions.amount_used` 字段或日志 `billing_source` 字段确认）
  - 在 10:00 时段发起请求，验证扣 B（行为不变）
- [x] 4.4 验证 `HasActiveUserSubscription` 与 `UserActiveSubscriptionsAllowWalletOverflow` 行为未受影响（这两个函数不应被改动；通过 grep 确认本次 PR 没有触碰它们）
