## ADDED Requirements

### Requirement: 渠道配置 token 路由参数

系统 SHALL 允许在每个 Channel 上配置两个新参数：
- `max_tokens`（int，默认 0）：渠道上下文硬上限（token 数）。0 表示不限。
- `token_tiers`（JSON 数组，默认空）：每条形如 `{"max_tokens": <int>, "priority_boost": <int64>}`，表示"当请求估算 token ≤ 此 max_tokens 时，把 priority_boost 叠加到该渠道的 base priority"。

未配置（默认值）时渠道 SHALL 表现与现状完全一致（`effective_priority = base_priority`，无 max_tokens 过滤）。

#### Scenario: 默认值兼容
- **WHEN** 一个渠道的 `max_tokens=0` 且 `token_tiers=[]`
- **THEN** 该渠道在所有请求下 `effective_priority = base_priority`，且永远不会因 token 过滤被排除

#### Scenario: max_tokens 配置
- **WHEN** 渠道配置 `max_tokens=200000`
- **THEN** 该渠道在持久化时存储该值，并通过 API 返回

#### Scenario: token_tiers 校验
- **WHEN** 提交 `token_tiers = [{max_tokens: 50000, priority_boost: 5}, {max_tokens: 200000, priority_boost: 3}]`
- **THEN** 系统接受配置，按数组顺序存储

#### Scenario: 非法 token_tiers 拒绝
- **WHEN** 提交的 tier 中 `max_tokens <= 0` 或 `priority_boost` 超出 [-100, 100]
- **THEN** 返回 400 错误，不持久化

---

### Requirement: 请求 token 估算

系统 SHALL 在 `Distribute` 中间件中、模型名解析成功后、调用 `CacheGetRandomSatisfiedChannel` 之前，对请求体做 token 估算，得到非负整数 `estTokens`。

估算 MUST 支持以下协议的消息文本提取：
- OpenAI Chat：`messages[*].content`（content 为 string 或 array of `{type:"text", text:"..."}`）
- OpenAI Completions：`prompt`（string 或 array）
- Anthropic Messages：`messages[*].content`（同上格式）
- Gemini：`contents[*].parts[*].text`
- Embeddings：`input`（string 或 array）

估算算法 SHALL 采用字符近似法：非 CJK 字符 4 字符 ≈ 1 token，CJK 字符 1.5 字符 ≈ 1 token。

估算 MUST 不修改请求体、不消耗 `c.Request.Body` 流（必须从 `common.GetBodyStorage(c)` 读取）。

#### Scenario: OpenAI Chat 估算
- **WHEN** 收到 OpenAI Chat 请求，`messages` 数组共 8000 字符（含 1000 个 CJK 字符）
- **THEN** 估算结果约为 `(8000-1000)/4 + 1000/1.5 = 1750 + 667 = 2417` tokens（允许 ±30% 误差）

#### Scenario: 多模态请求
- **WHEN** `messages[0].content` 是数组 `[{type:"text", text:"hello"}, {type:"image_url", ...}]`
- **THEN** 仅累加 text 部分字符数，非 text 部分跳过（不报错）

#### Scenario: 未知协议或空 body
- **WHEN** 请求体无法识别任何已知字段或为空
- **THEN** `estTokens = 0`，路由按原逻辑执行（不阻塞请求）

#### Scenario: 估算性能
- **WHEN** 请求体大小为 100KB
- **THEN** 估算耗时 < 1ms（中位数）

---

### Requirement: effective_priority 计算

系统 SHALL 在渠道选择时为每个候选渠道计算 `effective_priority = base_priority + Σ(token_tier.priority_boost where estTokens ≤ tier.max_tokens)`。

#### Scenario: 命中多个 tier
- **WHEN** 渠道配置 `token_tiers = [{max_tokens: 50000, boost: 5}, {max_tokens: 200000, boost: 3}]`，请求 `estTokens = 30000`
- **THEN** 该渠道 effective_priority = base + 5 + 3 = base + 8（两个 tier 都满足 30000 ≤ max_tokens）

#### Scenario: 仅命中较大 tier
- **WHEN** 同上渠道配置，请求 `estTokens = 100000`
- **THEN** 仅命中第二个 tier（100000 ≤ 200000），effective_priority = base + 3

#### Scenario: 无 tier 命中
- **WHEN** 同上渠道配置，请求 `estTokens = 300000`
- **THEN** 无 tier 命中，effective_priority = base + 0

---

### Requirement: max_tokens 软过滤

系统 SHALL 在 effective_priority 计算前，对候选渠道做软过滤：若渠道 `max_tokens > 0` 且 `estTokens > max_tokens`，则该渠道不参与本次选择。

过滤后候选集非空时，SHALL 仅在过滤后的集合上做后续 priority/weight 选择。

#### Scenario: 部分渠道超限
- **WHEN** 候选 [ch-A max=128K, ch-B max=200K, ch-C max=1M]，请求 estTokens=150K
- **THEN** ch-A 被排除，候选集 = [ch-B, ch-C]

#### Scenario: 全部渠道超限的软回退
- **WHEN** 候选 [ch-A max=128K, ch-B max=200K]，请求 estTokens=300K，全部超限
- **THEN** 回退到"忽略 max_tokens"模式，候选集恢复为全部渠道，按 effective_priority（boost=0）选择，并在响应 header 加 `X-Token-Routing-Fallback: max-tokens-exceeded`

#### Scenario: max_tokens 为 0 不参与过滤
- **WHEN** 渠道 `max_tokens = 0`，任意 estTokens
- **THEN** 该渠道永远不被 max_tokens 过滤排除

---

### Requirement: 选中渠道的选择算法

系统 SHALL 按 `effective_priority` DESC 对过滤后的候选渠道分 tier（unique effective_priorities），`retry` 索引选择第 `retry` 个 tier（0-indexed，超出范围则取最低 tier）。

选中 tier 后，在同 tier 的候选中按 `weight` 加权随机选取一个渠道（与现有 `GetRandomSatisfiedChannel` 同算法）。

#### Scenario: retry 跨 effective tier
- **WHEN** effective tiers 为 [13, 8, 1]，retry=0 选 tier 13，retry=1 选 tier 8
- **THEN** retry 递增时按 effective tier 降级

#### Scenario: 同 effective tier 内 weight 分流
- **WHEN** tier 10 内 [ch-A weight=50, ch-B weight=50]
- **THEN** ch-A 与 ch-B 各 50% 概率被选中

#### Scenario: 全部 effective tier 失败
- **WHEN** retry 超过 effective tier 数，所有候选渠道都返回错误
- **THEN** 返回最后一个尝试的错误（与现有行为一致），不引入新错误类型

---

### Requirement: Affinity token 守卫

系统 SHALL 在使用 affinity（粘性渠道）前检查：若 `estTokens > affinityChannel.max_tokens(>0)`，则跳过 affinity，走正常 token 路由。

#### Scenario: 亲和渠道容量足够
- **WHEN** 用户上次使用 ch-X（max=200K），本次请求 estTokens=50K
- **THEN** 复用 ch-X（保持亲和）

#### Scenario: 亲和渠道容量不足
- **WHEN** 用户上次使用 ch-X（max=128K），本次请求 estTokens=200K（如上传大文档）
- **THEN** 跳过 affinity，重新走 token 路由选择 200K+ 渠道

#### Scenario: 亲和渠道无 max_tokens 限制
- **WHEN** ch-X `max_tokens = 0`，任意 estTokens
- **THEN** 总是复用 ch-X（不受 token 守卫影响）

---

### Requirement: auto-group 跨组重试兼容

系统 SHALL 在 auto-group 模式下保持现有跨组重试逻辑，但 `priorityRetry` 基于 effective_priority tier 计算。

切换到下一组的条件不变：当前组的 effective priority tiers 全部 exhaust 后切下一组。

#### Scenario: 同组内 effective tier exhaust
- **WHEN** group A 有 effective tiers [13, 8]，retry 累积到 2 时切到 group B
- **THEN** group B 从其最高 effective tier 开始

#### Scenario: 跨组时 estTokens 不变
- **WHEN** 跨组切换发生
- **THEN** 新组内渠道的 effective_priority 仍基于同一 estTokens 计算

---

### Requirement: 渠道编辑页 token 路由配置 UI

前端 SHALL 在渠道编辑页（classic 与 default 双主题）提供「Token 路由策略」折叠面板，包含：
- `max_tokens` 数字输入框（0=不限，最小 0，最大 10000000）
- `token_tiers` 可增删行表格：每行 `max_tokens`（必填正整数）+ `priority_boost`（-100~100 整数）
- 「+ 添加分档」按钮
- 每行「删除」按钮
- 提示文案："estTokens 超过 max_tokens 的请求将跳过此渠道"

#### Scenario: 默认折叠
- **WHEN** 打开渠道编辑页
- **THEN** 「Token 路由策略」面板默认折叠，不占用视觉空间

#### Scenario: 加载已有配置
- **WHEN** 渠道已配置 `max_tokens=200000, token_tiers=[{50000, 5}]`
- **THEN** 展开面板时正确回显两个值

#### Scenario: 提交保存
- **WHEN** 用户填写 max_tokens=100000，添加两条 tier 后点击保存
- **THEN** 调用 PUT /api/channel 时 payload 包含这两个字段

---

### Requirement: 后端 API 契约向后兼容

Channel 的 GET/POST/PUT API SHALL 在响应 JSON 中包含 `max_tokens` 和 `token_tiers` 字段。

未升级的旧客户端发送不含这两个字段的请求时，系统 SHALL 按默认值（0 和空数组）处理，不报错。

#### Scenario: 旧客户端读
- **WHEN** 旧客户端 GET /api/channel/
- **THEN** 响应包含 max_tokens、token_tiers 字段（旧客户端忽略即可）

#### Scenario: 旧客户端写
- **WHEN** 旧客户端 POST /api/channel 不含 max_tokens、token_tiers
- **THEN** 系统按默认值 0 与 [] 持久化，返回 200
