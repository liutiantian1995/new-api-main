# 实施任务清单 — subscription-daily-time-window

> 按下方顺序推进；每个任务都应是可独立验证的。后端 JSON 必须用 `common.Marshal` / `common.Unmarshal`；SQL 必须兼容 SQLite / MySQL / PostgreSQL。

## 1. 后端模型字段与规范化

- [x] 1.1 在 `model/subscription.go` 的 `SubscriptionPlan` 结构体新增两个字段：`DailyActiveStartMinutes int` 与 `DailyActiveEndMinutes int`，GORM 标签 `gorm:"type:int;default:0"`，JSON tag 分别为 `daily_active_start_minutes` / `daily_active_end_minutes`。
- [x] 1.2 在 `UserSubscription` 结构体上新增同名字段，同样的 GORM/JSON 标签（这是窗口的快照字段）。
- [x] 1.3 扩展 `SubscriptionPlan.NormalizeDefaults()`：当 `DailyActiveStartMinutes == DailyActiveEndMinutes` 且非 0 时，两者归零；保证 start/end 均在 `[0, 1439]`（越界值钳位到合法区间并保留行为可预测——controller 层会先拦截，这里做兜底）。
- [x] 1.4 验证 `model/main.go` 中 `AutoMigrate` 列表包含 `&SubscriptionPlan{}` 与 `&UserSubscription{}`（若已是 AutoMigrate 全量，则无需改动；确认即可）。
- [x] 1.5 在 `model/subscription.go` 新增纯函数 `IsWithinDailyWindow(start, end, m int) bool`，按 design.md Decision 4 的签名实现：`start==end` 返回 `true`；`start<end` 返回 `m>=start && m<end`；`start>end` 返回 `m>=start || m<end`。

## 2. 后端扣费链路窗口校验

- [x] 2.1 在 `model/subscription.go` 新增本地 helper `currentMinuteOfDay() int`，基于 `time.Now().Local()` 返回 `hour*60 + minute`（0–1439）。
- [x] 2.2 修改 `PreConsumeUserSubscription`：在候选遍历循环里，紧跟现有"额度是否足够"判断之前，加 `if !IsWithinDailyWindow(sub.DailyActiveStartMinutes, sub.DailyActiveEndMinutes, currentMinuteOfDay()) { continue }`。语义保持"跳过该候选"，不要新增 error。
- [x] 2.3 修改 `HasActiveUserSubscription`（约 `model/subscription.go:1005`）：在 SQL 过滤基础上对每条候选同样执行窗口校验，窗口外的候选不计为"可用"。
- [x] 2.4 检查 `GetAllActiveUserSubscriptions`（约 `model/subscription.go:988`）返回给前端的"活跃订阅列表"——决定是否在返回前过滤窗口外订阅；按 design 决策，本函数继续返回所有 active 订阅但附带原始窗口字段，让前端在 UI 上自行展示"当前不在窗口内"（不在本函数过滤，避免改变 API 形状）。
- [x] 2.5 在创建 `UserSubscription`（订单完成、管理端手动发放、`PendingSubscriptionActivation` 激活）的所有路径上，从套餐快照这两个字段；定位所有 `UserSubscription{...}` 字面量构造点，补充字段拷贝。

## 3. 后端 Controller 校验

- [x] 3.1 在 `controller/subscription.go` 创建/更新套餐的 handler 中，对请求 body 的 `daily_active_start_minutes` / `daily_active_end_minutes` 做 `0 <= v <= 1439` 校验；越界返回 400 与清晰错误信息（使用 i18n 后端 key）。
- [x] 3.2 后端 i18n：在 `i18n/locales/en.json` 与 `i18n/locales/zh.json`（后端目录）新增对应错误文案 key，例如 `subscription daily window out of range`。

## 4. 后端测试

- [x] 4.1 新增 `model/subscription_window_test.go`（或扩展现有测试文件）：表驱动测试 `IsWithinDailyWindow`，覆盖 全天(0/0)、同日窗口边界含首不含尾、跨午夜窗口（23:00–06:00）的 22:59/23:00/00:00/05:00/06:00/10:00 等关键点。
- [x] 4.2 新增 `NormalizeDefaults` 等值归零的单元测试（`600/600` → `0/0`；`1380/360` 保持不变）。
- [x] 4.3 为 `PreConsumeUserSubscription` 增加表驱动测试：构造一个窗口为 23:00–06:00、额度充足的订阅，验证当前时间在 02:30 时被选中、在 12:00 时被跳过并最终返回"无可用订阅"。
- [x] 4.4 用 `testify/require` 做 setup 与 fatal 断言、`testify/assert` 做非致命检查（按 `AGENTS.md` 要求）。

## 5. 前端类型与表单

- [x] 5.1 在 `web/default/src/features/subscriptions/types.ts` 的 `subscriptionPlanSchema` 与 `userSubscriptionSchema` 上新增字段 `daily_active_start_minutes: z.number().int().min(0).max(1439).default(0)` 与 `daily_active_end_minutes`（同样约束）。
- [x] 5.2 在 `lib.ts`（或订阅模块的表单 schema 文件）扩展 `getPlanFormSchema` / `PLAN_FORM_DEFAULTS` / `planToFormValues` / `formValuesToPlanPayload`：表单内部用 `dailyActiveStart: string | null`（HH:mm）和 `dailyActiveEnd: string | null`，提交时换算成分钟；两者要么都填要么都空。
- [x] 5.3 表单校验：部分填写（一空一非空）触发错误，文案走 i18n。

## 6. 前端管理端 UI

- [x] 6.1 在 `subscriptions-mutate-drawer.tsx` 的表单中新增"每日可用时段"区块：两个 `<input type="time">`（或 Base UI 的等价组件），分别绑定 `dailyActiveStart` / `dailyActiveEnd`。
- [x] 6.2 增加"清空（全天）"按钮，把两字段置 `null`。
- [x] 6.3 当 `start > end`（按 HH:mm 字符串比较，或转分钟比较）时，显示提示文案 `This window crosses midnight and will expire at {{end}} the next day.`，通过 `t(...)` + i18next 的 interpolation 渲染。
- [x] 6.4 在窗口区块下方常驻显示共存语义提示 `Outside this window, other available subscriptions will still be consumed in expiry order.`，让运营方知晓多订阅共存时的扣费顺序（参见 design Decision 8）。
- [x] 6.5 提交按钮保留现有逻辑；新增字段的 form schema 校验通过后才允许提交。

## 7. 前端用户端展示

- [x] 7.1 在 `user-subscriptions-dialog.tsx`（或用户端展示订阅列表的位置）：当订阅的 `daily_active_start_minutes !== daily_active_end_minutes` 或非同时为 0 时，展示 `Daily {{start}}–{{end}} available`（用客户端格式化分钟→HH:mm）。
- [x] 7.2 当两值同时为 0 时不显示该行（避免噪音）。

## 8. 前端 i18n 与构建

- [x] 8.1 在 `web/default/src/i18n/locales/en.json` 与 `zh.json` 新增 design Decision 7 列出的全部 key；其余语言（fr/ru/ja/vi）至少保留英文 source（由后续 `bun run i18n:sync` 维护）。
- [x] 8.2 在 `web/default/` 下运行 `bun run build` 验证构建通过、无类型错误。
- [x] 8.3 在 `web/default/` 下运行 `bun run i18n:sync`（如可用）补齐其余语言的占位。

## 9. 文档与回归

- [x] 9.1 更新 `CLAUDE.md` / `AGENTS.md` 如有相关说明（仅在必要时；通常本特性不需要修改项目级约定）。
- [x] 9.2 在 `openspec/changes/subscription-daily-time-window/` 目录下确认 proposal/design/specs/tasks 四件套齐全且自洽。
- [x] 9.3 端到端手测脚本：本地起服务 → 管理端给套餐 P 配置 23:00–06:00 → 用户购买 → 在窗口内调用 `/v1/chat/completions` 走订阅扣费 → 修改服务器时间到 12:00 再次调用 → 验证回落到钱包、日志 `billing_source` 变为 `wallet`。
- [x] 9.4 验证数据库迁移：分别在 SQLite（默认）、MySQL、PostgreSQL（若本地有环境，至少 SQLite 必须过）上启动新版本，确认 4 列被添加、默认 0、旧行无破坏。
- [x] 9.5 回归：确认既有"全天套餐"在升级后行为完全不变（窗口 0/0 时 `IsWithinDailyWindow` 永远返回 true，扣费路径与变更前一致）。
