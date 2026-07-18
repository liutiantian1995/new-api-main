## Context

new-api 当前的渠道选择位于 `middleware/distributor.go` 的 `Distribute` 中间件，调用链：
`Distribute → CacheGetRandomSatisfiedChannel(service/channel_select.go) → GetRandomSatisfiedChannel(model/channel_cache.go)`

选择维度只有 `priority + weight + retry`：
- `group2model2channels` 按 base_priority DESC 排序缓存
- retry 跨 priority tier（高 tier 全失败才降级）
- 同 tier 内按 weight 加权随机

`getModelFromJSONBody` 仅用 `gjson.GetManyBytes(requestBody, "model", "group")` 提取模型名与分组，**不解析 messages**，因此路由对请求大小"盲目"。Body 已通过 `common.GetBodyStorage(c)` 提供可重复读的字节切片。

约束：
- 必须支持 SQLite / MySQL / PostgreSQL 三种数据库
- 必须保持双前端主题（classic Semi Design + default Base UI/Tailwind）功能对等
- 不破坏 auto-group 跨组重试、affinity、模型重定向等现有特性
- 不引入新的硬阻塞路径——配置错误或估算偏差不能导致请求失败

## Goals / Non-Goals

**Goals:**
- 让运维能基于请求大小做渠道偏好偏移（小请求偏好廉价渠道，大请求流向大容量渠道）
- 与现有 priority/weight 体系协作而非取代：boost 是修饰，max_tokens 是软过滤
- 单请求路由开销增量 <1ms
- 向后兼容：未配置的渠道行为完全同现状
- 故障容错：所有 max_tokens 都超限时软回退，不死锁

**Non-Goals:**
- 不实现精确 tokenizer（如 tiktoken）——字符近似法足够分级路由
- 不估算 output tokens——路由仅基于 input
- 不修改计费系统——估算值不参与计费
- 不修改 token 表、用户表
- 不实现多模态精确计数（图片/音频）——v1 仅算文本部分，标注 `text-only`
- 不引入新的依赖（gjson 已是项目依赖）

## Decisions

### Decision 1: 字符近似法估算 token，而非精确 tokenizer

**选择**：非 CJK 4 字符 ≈ 1 token，CJK 1.5 字符 ≈ 1 token。

**理由**：
- tiktoken Go 移植版 +2MB 依赖、3-15ms 延迟，分级路由不值得
- 分级边界本身配 buffer（如 180K 而非 200K 切档），±30% 误差可吸收
- 字符近似法 <1ms，纯函数无状态

**Alternative**：完整 BPE tokenizer——精度高但代价过大，且 OpenAI/Anthropic/Gemini 各自 tokenizer 不同，统一不现实。

### Decision 2: effective_priority 运行时计算，不污染缓存结构

**选择**：`group2model2channels` 仍按 base_priority DESC 排序存储。每次 `GetRandomSatisfiedChannel` 调用时，对每个候选渠道运行时计算 effective_priority，重新分组 tier。

**理由**：
- 缓存结构不变，`InitChannelCache` 仅多解析一个 JSON 字段
- effective_priority 依赖 estTokens（每请求不同），无法预缓存
- 候选渠道数通常 <50，运行时计算开销可忽略（<0.1ms）

**Alternative**：缓存按 effective_priority 分桶——需要按 estTokens 维度建多份缓存，复杂度暴增，不值得。

### Decision 3: max_tokens 软过滤 + 全空回退，而非硬拒绝

**选择**：过滤后候选集为空时，回退到"忽略 max_tokens"的全集合，按 effective_priority（boost=0）选择，加 `X-Token-Routing-Fallback` 响应 header。

**理由**：
- 硬拒绝会导致配置一改全挂的事故
- 软回退最差情况是选到上游会 413 的渠道，但比直接 503 好——上游可能正好容忍
- 响应 header 让运维可观测回退频率

**Alternative**：硬拒绝并返回 413——违反"不堵塞"承诺，否决。

### Decision 4: tier 语义用 "≤ max_tokens 命中"（语义 B）

**选择**：每条 tier `{max_tokens, priority_boost}` 表示 `estTokens ≤ max_tokens` 时叠加 boost。多个 tier 可同时命中，boost 累加。

**理由**：
- 配置直观："我喜欢处理 ≤50K 的请求，加 +5；≤200K 也行，加 +3"
- 多 tier 累加允许表达"越小越偏好"的渐进语义

**Alternative（语义 A）**：区间命中 `[prev_max, this.max)`——配置时需保持区间连续，易出错，否决。

### Decision 5: token 估算放在 Distribute 内，而非独立中间件

**选择**：在 `Distribute()` 解析 model 成功后立即估算，不新增 middleware。

**理由**：
- 复用 `common.GetBodyStorage(c)`，无需重复读 body
- 仅在确定走模型路由后才计 token，避免无谓开销（如 MJ/Suno 等非文本路径可跳过）
- 减少中间件链复杂度

**Alternative**：独立 `TokenEstimator` 中间件——多一层间接，且需处理"未走 Distribute"的边界，不值得。

### Decision 6: Affinity token 守卫在 Distribute 内做

**选择**：复用现有 affinity 命中逻辑，但在返回 affinity 渠道前加 `estTokens > max_tokens` 检查，超限则弃用 affinity。

**理由**：
- 改动最小，不重写 affinity 机制
- 守卫失败优雅降级到正常 token 路由

### Decision 7: RetryParam 新增 EstTokens 字段透传

**选择**：`service/channel_select.go` 的 `RetryParam` 新增 `EstTokens int`，`CacheGetRandomSatisfiedChannel` 调用 `GetRandomSatisfiedChannel` 时传入。

**理由**：
- 保持 service 层与 model 层的边界
- auto-group 跨组迭代时使用同一 estTokens（请求未变）

### Decision 8: 数据库字段用 GORM 标签，不写迁移脚本

**选择**：Channel struct 新增：
```go
MaxTokens  int        `json:"max_tokens" gorm:"default:0"`
TokenTiers []TokenTier `json:"token_tiers" gorm:"serializer:json;type:text"`
```

**理由**：
- 项目约定用 GORM AutoMigrate，三种 DB 自动处理新增列
- `serializer:json` 让 GORM 自动序列化/反序列化，无需手写 JSON 列处理
- `type:text` 在三种 DB 都兼容（避免 PostgreSQL JSON 列类型与 MySQL/SQLite 差异）

### Decision 9: 前端 UI 默认折叠

**选择**：「Token 路由策略」作为折叠面板，默认收起。

**理由**：
- 大多数渠道不需要配置，避免视觉噪音
- 高级用户展开即可配置

## Risks / Trade-offs

- **[估算误差 30%]** → tier 边界配在实际容量的 70% 处（如 200K 渠道配 max_tokens=140K），文档明确建议
- **[多模态请求计数偏低]** → 仅算文本部分，图片/音频 token 被忽略，可能导致图片请求被低估走到小容量渠道。在响应 header 加 `X-Token-Estimate: text-only`，运维可观测
- **[配置错误导致回退频繁]** → 监控 `X-Token-Routing-Fallback` header 出现率，超过阈值告警
- **[effective_priority 逆转 base priority]** → 这是设计意图（boost 可让低 base 渠道升到高 effective），但需文档明确说明，避免运维误解
- **[多 tier 累加导致 effective_priority 溢出]** → boost 范围限制 [-100, 100]，tier 数量实际不会超过 5 个，int64 足够
- **[缓存加载时 token_tiers JSON 解析失败]** → 解析失败时按空数组处理，记录 warning 日志，不阻塞缓存初始化

## Migration Plan

1. **后端先行**：Channel 字段 + 估算 + 选择算法 + API 契约（向后兼容，旧前端不破）
2. **前端跟进**：双主题 UI 同步上线
3. **灰度**：先在测试环境配置 1-2 个渠道验证，观察 `X-Token-Routing-Fallback` 频率
4. **回滚**：若发现严重问题，把所有渠道的 `max_tokens` 重置为 0、`token_tiers` 清空，等同回到现状（无需代码回滚）

## Open Questions

- 是否需要管理员全局开关（一键禁用 token 路由）？倾向不需要，因为清空配置即可达到效果
- 估算结果是否记入日志？倾向仅在 debug 级别输出，不进结构化日志
- 1M 渠道是否需要标"独占模式"（小请求绝不走它）？当前 boost=-3 已足够，不需要硬独占
