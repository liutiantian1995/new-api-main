## ADDED Requirements

### Requirement: 订阅套餐每日可用时段字段定义

系统 SHALL 在 `SubscriptionPlan`（套餐模板）上提供两个整数字段用于表达"每日可用时段"：

- `daily_active_start_minutes`：自当日午夜 0:00 起的分钟偏移（含），取值范围 0–1439。
- `daily_active_end_minutes`：自当日午夜 0:00 起的分钟偏移（不含），取值范围 0–1439。

字段语义：

- `start == end`（含两者都为 0）MUST 被规范化为 `0/0`，表示"全天可用，无日内限制"。
- `start < end`（如 1380/360 不成立；示例为 `start=0, end=360` 表示 00:00–06:00）表示同日窗口。
- `start > end`（如 `start=1380`（23:00）, `end=360`（06:00））MUST 被系统接受并表示"跨午夜窗口"：从 `start` 起当日时段生效，到次日 `end` 失效。
- 两个值 MUST 各自落在 `[0, 1439]` 区间内；越界值 MUST 在写入（创建/更新套餐）时被拒绝（返回 4xx 错误）。

字段 MUST 默认值为 0，向后兼容——升级前已存在的套餐与未配置时段的套餐 MUST 表现为"全天可用"。

#### Scenario: 默认值表示全天可用

- **WHEN** 运营方创建一个套餐但未提供 `daily_active_start_minutes` / `daily_active_end_minutes`
- **THEN** 系统 MUST 将两者存储为 `0`
- **AND** 该套餐 MUST 在一日内的任何时刻都被视为"在窗口内"

#### Scenario: 同日窗口规范化

- **WHEN** 运营方提交 `daily_active_start_minutes=0` 且 `daily_active_end_minutes=360`（00:00–06:00）
- **THEN** 系统 MUST 接受该取值并按原值存储
- **AND** 该套餐 MUST 仅在每日 00:00:00（含）至 06:00:00（不含）之间被视为可用

#### Scenario: 跨午夜窗口规范化

- **WHEN** 运营方提交 `daily_active_start_minutes=1380`（23:00）且 `daily_active_end_minutes=360`（06:00）
- **THEN** 系统 MUST 接受该取值并按原值存储（不做翻转或拆分）
- **AND** 该套餐 MUST 在每日 23:00:00（含）至次日 06:00:00（不含）之间被视为可用

#### Scenario: 非法取值被拒绝

- **WHEN** 运营方提交 `daily_active_start_minutes=-1` 或 `daily_active_end_minutes=1440` 或任何超出 `[0, 1439]` 的值
- **THEN** 系统 MUST 拒绝创建/更新并返回 4xx 错误
- **AND** 错误信息 MUST 明确指出字段名与合法范围

#### Scenario: 相等值等同于全天

- **WHEN** 运营方提交 `daily_active_start_minutes=600` 且 `daily_active_end_minutes=600`（两者相等且非 0）
- **THEN** 系统 MUST 将两者规范化为 `0/0` 存储
- **AND** 该套餐 MUST 表现为"全天可用"

---

### Requirement: 用户订阅实例快照每日时段字段

当系统创建一条新的 `UserSubscription` 记录（无论是订单完成、管理端手动发放、还是激活等待中的订阅）时，系统 MUST 将对应 `SubscriptionPlan` 的 `daily_active_start_minutes` 与 `daily_active_end_minutes` 原值快照到该用户订阅实例上，并随后的扣费校验 MUST 只读用户订阅实例上的快照字段。

此要求确保套餐后续被运营方修改时，已购用户的契约保持稳定，与现有的 `UpgradeGroup` / `DowngradeGroup` / `AllowWalletOverflow` 快照模式一致。

#### Scenario: 购买套餐时快照窗口字段

- **WHEN** 用户购买套餐 P（`daily_active_start_minutes=1380`, `daily_active_end_minutes=360`）成功
- **THEN** 新建的 `UserSubscription` 记录 MUST 包含 `daily_active_start_minutes=1380` 与 `daily_active_end_minutes=360`
- **AND** 后续扣费校验 MUST 读取该用户订阅实例上的快照值

#### Scenario: 套餐修改不影响存量订阅

- **WHEN** 用户 U 已持有一条快照为 `1380/360` 的 `UserSubscription`
- **AND** 运营方将套餐 P 的窗口修改为 `0/0`（全天）
- **THEN** 用户 U 的该订阅 MUST 继续按 `1380/360` 的窗口被校验，直到该订阅过期或被显式更新

---

### Requirement: 扣费时强制校验每日窗口

`PreConsumeUserSubscription`（订阅预扣费）MUST 在评估每条候选订阅时执行"当前时刻是否落在该订阅的每日窗口内"的校验：

- 当前时刻在窗口内：候选订阅可被选中，继续走现有"额度是否足够"的判断。
- 当前时刻不在窗口内：MUST 跳过该候选（不返回错误，不消耗预扣记录），与"额度不足时跳过"的降级语义保持一致。

当所有候选订阅都被日内窗口排除（或同时被额度/状态排除）时，`PreConsumeUserSubscription` MUST 返回现有的"无可用订阅"错误（不引入新的错误码），使上层按现有路径回落到钱包或返回标准错误。

`HasActiveUserSubscription`（用于 token 鉴权时的"是否存在可用订阅"预检）MUST 同样把"当前时刻在窗口外"的订阅视为不可用——即只把"在窗口内且状态 active 且未过期"的订阅计为可用。

#### Scenario: 窗口内请求正常扣费

- **WHEN** 用户 U 持有订阅 S（窗口 23:00–06:00，状态 active，额度充足）
- **AND** 当前本地时间为 02:30
- **THEN** `PreConsumeUserSubscription` MUST 选中 S 并按现有逻辑预扣额度

#### Scenario: 窗口外请求跳过订阅并降级

- **WHEN** 用户 U 持有订阅 S（窗口 23:00–06:00，状态 active，额度充足）
- **AND** 当前本地时间为 12:00
- **THEN** `PreConsumeUserSubscription` MUST 跳过 S
- **AND** 若用户无其他可用订阅，调用 MUST 返回"无可用订阅"错误
- **AND** 上层（`FundingSource` 选择链路）MUST 按现有路径回落到钱包余额扣费

#### Scenario: 多订阅按 end_time 顺序评估但受窗口过滤

- **WHEN** 用户持有订阅 A（窗口 00:00–06:00，end_time 较早）和订阅 B（窗口 18:00–24:00 即 1080/0 规范化为全天或 1080/1439，end_time 较晚）
- **AND** 当前时间为 20:00
- **THEN** 系统 MUST 仍按 `end_time asc, id asc` 遍历候选
- **AND** 跳过 A（不在 A 的窗口），选中第一个在窗口内且额度充足的候选

#### Scenario: 窗口外请求会消耗同时持有的全天订阅

- **WHEN** 用户持有订阅 A（每日窗口 11:00–23:00，end_time 较早）和订阅 B（全天 0/0，end_time 较晚）
- **AND** 当前时间为 02:00（在 A 的窗口外）
- **THEN** 系统 MUST 按 `end_time asc` 先评估 A 并跳过（窗口外）
- **AND** 继续评估 B（全天，在窗口内）→ 若额度充足 MUST 选中 B 并扣 B 的额度
- **AND** 调用 MUST NOT 因 A 在窗口外就直接回落到钱包而绕过 B

#### Scenario: 用户只持有窗口订阅且在窗口外才回落钱包

- **WHEN** 用户仅持有订阅 A（每日窗口 11:00–23:00），当前时间为 02:00
- **AND** 用户没有其他任何 active 且未过期的订阅
- **THEN** 系统 MUST 跳过 A 后返回"无可用订阅"错误
- **AND** 上层 MUST 按现有路径回落到钱包余额扣费

#### Scenario: 全天订阅永远在窗口内

- **WHEN** 用户持有的订阅快照为 `0/0`（全天）
- **THEN** 任意当前时刻 MUST 被视为在窗口内
- **AND** 该订阅的扣费行为 MUST 与本次变更前的实现完全一致

#### Scenario: 预检不把窗口外订阅计为可用

- **WHEN** 用户仅持有订阅 S（窗口 23:00–06:00），当前时间为 12:00
- **THEN** `HasActiveUserSubscription` MUST 返回 `false`
- **AND** token 鉴权链路 MUST 表现为"该用户当前无可用订阅"（与无订阅等价处理）

---

### Requirement: 每日窗口判定算法

系统 MUST 提供一个共享的纯函数用于判定"给定当前时刻是否落在窗口内"，并被所有需要校验的代码路径复用。设 `start` / `end` 为规范化后的分钟偏移，`now` 为当前本地时间分钟数 `m = hour*60 + minute`（0–1439）：

- 若 `start == end`（含 0/0）：MUST 返回 `true`（全天可用）。
- 若 `start < end`：MUST 返回 `start <= m < end`。
- 若 `start > end`（跨午夜）：MUST 返回 `m >= start OR m < end`。

该算法 MUST 不依赖时区转换——`now` 统一使用服务进程本地时区（与现有 `time.Now()` 一致），并在文档/UI 文案中明确说明"时间为服务器本地时间"。

#### Scenario: 全天判定

- **WHEN** `start=0, end=0, m=0`
- **THEN** 判定函数 MUST 返回 `true`

#### Scenario: 同日窗口边界含首不含尾

- **WHEN** `start=0, end=360`
- **AND** `m=0`（含）
- **OR** `m=359`
- **THEN** 判定函数 MUST 返回 `true`
- **WHEN** `m=360`
- **THEN** 判定函数 MUST 返回 `false`

#### Scenario: 跨午夜窗口

- **WHEN** `start=1380, end=360`
- **AND** `m=1380`（23:00，含）
- **OR** `m=0`（00:00）
- **OR** `m=300`（05:00）
- **THEN** 判定函数 MUST 返回 `true`
- **WHEN** `m=600`（10:00）
- **OR** `m=1379`（22:59）
- **THEN** 判定函数 MUST 返回 `false`

---

### Requirement: 管理端表单支持配置每日窗口

管理端订阅套餐编辑抽屉（`SubscriptionsMutateDrawer`）MUST 提供：

- 两个时间选择器（HH:mm 24 小时制），分别绑定套餐的"每日生效时间"与"每日失效时间"。
- 一个清空按钮/重置选项，将两个字段置空（=全天）。
- 当 `start > end`（HH:mm 比较）时，UI MUST 显示提示"该窗口跨午夜，将在次日 end 时间失效"以提醒运营方。
- 表单提交时 MUST 把 HH:mm 转换为分钟偏移（`H*60 + M`）后下发到后端。

表单校验：

- 两个时间要么都填，要么都不填（部分填写 MUST 触发表单校验错误）。
- 任意时间超出 23:59 MUST 被时间组件自身约束（不可能发生）。

#### Scenario: 配置跨午夜窗口

- **WHEN** 管理员在编辑抽屉里将"每日生效时间"设为 23:00、"每日失效时间"设为 06:00
- **THEN** UI MUST 显示"跨午夜"提示
- **AND** 提交时下发的 payload MUST 包含 `daily_active_start_minutes=1380` 与 `daily_active_end_minutes=360`

#### Scenario: 清空表示全天

- **WHEN** 管理员点击清空按钮或两字段均留空
- **THEN** 提交时下发的 payload MUST 包含 `daily_active_start_minutes=0` 与 `daily_active_end_minutes=0`

#### Scenario: 仅填写一边被拦截

- **WHEN** 管理员只填写了"每日生效时间"而"每日失效时间"留空
- **THEN** 表单 MUST 校验失败并阻止提交
- **AND** 错误信息 MUST 同时出现在管理端与 i18n 文案中

---

### Requirement: 用户端展示订阅每日窗口

用户端在展示用户已持有的订阅时（如 `user-subscriptions-dialog`），若订阅的窗口不是全天（即 `start != end` 且不同时为 0），MUST 在订阅卡片/行上以人类可读格式展示该窗口（例如 `每日 23:00–06:00 可用`），帮助用户理解为何某次请求会回落到钱包。

#### Scenario: 全天订阅不展示窗口

- **WHEN** 用户持有的订阅 `daily_active_start_minutes=0` 且 `daily_active_end_minutes=0`
- **THEN** UI MUST 不显示"每日窗口"相关文案

#### Scenario: 跨午夜窗口以本地时间展示

- **WHEN** 用户持有的订阅 `daily_active_start_minutes=1380` 且 `daily_active_end_minutes=360`
- **THEN** UI MUST 显示形如 `每日 23:00–06:00 可用` 的文案
- **AND** 文案 MUST 通过 i18n 系统（`t(...)`）获取，不硬编码

---

### Requirement: 数据库迁移向后兼容

新增字段的数据库迁移 MUST 满足：

- 通过 GORM `AutoMigrate` 自动添加新列，列默认值为 0。
- 迁移 MUST 在 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6 三个数据库上同时成功。
- 迁移 MUST 保留所有既有行与字段值不变。
- 迁移后，所有既有套餐与既有用户订阅实例 MUST 表现为"全天可用"（因为新列默认 0）。

#### Scenario: 在 SQLite 上首次迁移

- **WHEN** 在一个已有数据的 SQLite 数据库上启动新版本
- **THEN** GORM `AutoMigrate` MUST 成功添加 4 列（计划表 2 列 + 用户订阅表 2 列）
- **AND** 既有行的新列值 MUST 为 0
- **AND** 现有订阅扣费行为 MUST 与升级前完全一致

#### Scenario: 在 MySQL 与 PostgreSQL 上重复启动

- **WHEN** 在 MySQL 或 PostgreSQL 上重复启动服务（`AutoMigrate` 幂等执行）
- **THEN** 系统 MUST NOT 重复执行 `ALTER TABLE`（GORM 检测列已存在）
- **AND** 不产生任何错误日志
