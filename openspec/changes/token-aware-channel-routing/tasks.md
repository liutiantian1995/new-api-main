## 1. 后端数据模型

- [x] 1.1 在 `model/channel.go` 的 Channel struct 新增 `MaxTokens int` 字段（gorm:`default:0`）与 `TokenTiers []TokenTier` 字段（gorm:`serializer:json;type:text`）
- [x] 1.2 定义 `TokenTier` struct：`MaxTokens int` + `PriorityBoost int64`，JSON tag `max_tokens`/`priority_boost`
- [x] 1.3 在 Channel 的 `Insert`/`Update`/`GetAllChannels` 等方法中确认 GORM 自动序列化 token_tiers（无需手写 JSON 处理）
- [x] 1.4 验证 AutoMigrate 在 SQLite/MySQL/PostgreSQL 三种 DB 下正确新增列（运行现有 migration 测试）

## 2. 后端 token 估算

- [x] 2.1 新建 `service/token_estimate.go`，实现 `EstimateInputTokens(body []byte) int`
- [x] 2.2 实现 OpenAI Chat 协议解析：`messages[*].content` 支持 string 与 array of `{type:"text",text:"..."}`
- [x] 2.3 实现 OpenAI Completions 解析：`prompt` 支持 string 与 array
- [x] 2.4 实现 Anthropic Messages 解析：复用 OpenAI Chat 解析逻辑
- [x] 2.5 实现 Gemini 解析：`contents[*].parts[*].text`
- [x] 2.6 实现 Embeddings 解析：`input` 支持 string 与 array
- [x] 2.7 实现字符近似算法：非 CJK 4 字符≈1 token，CJK 1.5 字符≈1 token
- [x] 2.8 未知协议或空 body 时返回 0（不报错）
- [x] 2.9 新建 `service/token_estimate_test.go`，覆盖各协议、多模态、空 body、纯 CJK、纯英文、混合文本用例

## 3. 后端渠道选择算法

- [x] 3.1 修改 `model/channel_cache.go` 的 `GetRandomSatisfiedChannel` 签名：新增 `estTokens int` 参数
- [x] 3.2 在 `InitChannelCache` 中加载 channel 的 `MaxTokens` 与 `TokenTiers` 到缓存（channelsIDM map）
- [x] 3.3 实现软过滤：`estTokens > ch.MaxTokens(>0)` 的渠道从候选集排除
- [x] 3.4 实现全空回退：过滤后候选集为空时，恢复全集合并在响应 header 设置 `X-Token-Routing-Fallback: max-tokens-exceeded`
- [x] 3.5 实现 effective_priority 计算：`base + Σ(boost where estTokens ≤ tier.max_tokens)`
- [x] 3.6 按 effective_priority DESC 重新分 tier（unique values），retry 索引选择
- [x] 3.7 同 effective tier 内沿用现有 weight 加权随机算法
- [x] 3.8 新建/扩展 `model/channel_cache_test.go`，覆盖：默认值兼容、单 tier 命中、多 tier 累加、max_tokens 软过滤、全空回退、retry 跨 effective tier、weight 分流

## 4. 后端 service 层透传

- [x] 4.1 在 `service/channel_select.go` 的 `RetryParam` struct 新增 `EstTokens int` 字段
- [x] 4.2 `CacheGetRandomSatisfiedChannel` 调用 `GetRandomSatisfiedChannel` 时传入 `param.EstTokens`
- [x] 4.3 auto-group 跨组迭代时使用同一 EstTokens（请求未变，无需重算）
- [x] 4.4 扩展 `service/channel_select_test.go` 覆盖 estTokens 透传

## 5. 后端 Distribute 中间件

- [x] 5.1 在 `middleware/distributor.go` 的 `Distribute` 中，model 解析成功后调用 `service.EstimateInputTokens(body)`
- [x] 5.2 把 estTokens 存入 context（`constant.ContextKeyEstimatedTokens`）供后续观测
- [x] 5.3 在 affinity 命中分支前加 token 守卫：`estTokens > affinityCh.MaxTokens(>0)` 时跳过 affinity
- [x] 5.4 调用 `CacheGetRandomSatisfiedChannel` 时构造 RetryParam 包含 EstTokens
- [x] 5.5 在 debug 日志输出 estTokens 与选中渠道的 effective_priority（便于排障）
- [x] 5.6 非文本路径（MJ/Suno/audio/images）跳过估算（estTokens=0）

## 6. 后端 API 契约

- [x] 6.1 确认 Channel GET/POST/PUT 自动包含 `max_tokens` 与 `token_tiers`（GORM + JSON tag 自动）
- [x] 6.2 在 Channel controller 的写入接口加输入校验：`max_tokens >= 0`、每个 tier `max_tokens > 0`、`priority_boost ∈ [-100, 100]`
- [x] 6.3 非法输入返回 400 错误信息
- [x] 6.4 旧客户端不含字段时按默认值 0/[] 持久化（GORM 默认值处理）
- [x] 6.5 在 controller 测试中覆盖：合法配置、非法配置、缺失字段

## 7. 前端 classic 主题

- [x] 7.1 在 `web/classic/src/components/edit_channel/` 找到现有多区域编辑组件结构
- [x] 7.2 新增「Token 路由策略」折叠面板组件（默认收起）
- [x] 7.3 实现 max_tokens 输入框（NumberInput，0=不限）
- [x] 7.4 实现 token_tiers 可增删行表格：每行 max_tokens + priority_boost 输入框
- [x] 7.5 「+ 添加分档」按钮与每行「删除」按钮
- [x] 7.6 提示文案："estTokens 超过 max_tokens 的请求将跳过此渠道"
- [x] 7.7 编辑页加载时正确回显已有配置
- [x] 7.8 保存时把两个字段加入 PUT /api/channel payload

## 8. 前端 default 主题

- [x] 8.1 在 `web/default/src/features/channels/` 找到渠道编辑组件结构
- [x] 8.2 用 Base UI Accordion + Tailwind 实现等价面板
- [x] 8.3 同 classic 实现 max_tokens 输入与 token_tiers 行表格
- [x] 8.4 加载回显与保存逻辑
- [x] 8.5 与 default 主题其他面板视觉风格一致

## 9. i18n 文案

- [x] 9.1 在 `web/default/src/i18n/locales/en.json` 与 `zh.json` 添加新 key：`Token Routing Strategy`、`Max Tokens`、`Token Tiers`、`Add Tier`、`estTokens exceeds max_tokens will skip this channel` 等
- [x] 9.2 classic 主题 i18n 文案同步
- [x] 9.3 运行 `bun run i18n:sync` 校验 key 完整性

## 10. 集成验证

- [ ] 10.1 启动开发环境，创建渠道配 max_tokens=128000 + token_tiers=[{50000, 5}]
- [ ] 10.2 发送 10K 请求，验证走 boost 渠道
- [ ] 10.3 发送 80K 请求，验证走 128K 渠道（未超限）
- [ ] 10.4 发送 200K 请求，验证 128K 渠道被过滤，走其他渠道
- [ ] 10.5 发送 5M 请求，验证全空回退 + `X-Token-Routing-Fallback` header
- [ ] 10.6 关闭一个高 effective 渠道，验证 retry 跨 effective tier 降级
- [ ] 10.7 验证 affinity token 守卫：小请求建亲和到大容量渠道后，发大请求应弃用亲和
- [ ] 10.8 验证 auto-group 跨组场景下 estTokens 透传正确

## 11. 性能与回归

- [x] 11.1 基准测试：100KB body 估算耗时（目标 <1ms 中位数）
- [x] 11.2 基准测试：50 候选渠道的 effective_priority 计算耗时（目标 <0.1ms）
- [x] 11.3 回归测试：所有渠道未配置 token_tiers 时，行为完全同 main 分支
- [x] 11.4 回归测试：现有 channel_cache_test、distributor_test 全部通过
- [x] 11.5 三种 DB（SQLite/MySQL/PostgreSQL）跑 migration 测试

## 12. 文档与发布

- [x] 12.1 在 `docker-build-ops.md` 或单独文档补充 token 路由策略说明（tier 边界 buffer 建议、boost 量级参考）
- [x] 12.2 更新 VERSION 文件（v1.0.0-rc.15-batch4）
- [x] 12.3 构建镜像并验证（不使用 --no-cache）— v1.0.0-rc.15-batch4 已推送
- [ ] 12.4 提交 PR，PR body 说明配置示例与回滚方式（清空配置即回滚）

## 13. 代码审查问题修复

- [x] 13.1 **C-1（CRITICAL）**：删除包级 `tokenRoutingFallbackSeen` 与 `ConsumeTokenRoutingFallback`，改 `GetRandomSatisfiedChannel` / `CacheGetRandomSatisfiedChannel` 多返回一个 `fallback bool`，消除数据竞争与跨请求串扰
- [x] 13.2 **H-1**：`controller/channel.go validateChannel` 加 `len(channel.TokenTiers) <= 10` 校验，并补单元测试覆盖 10/11 边界
- [x] 13.3 **H-2**：Classic 主题 `TokenRoutingPanel` 移除 `Form.InputNumber field='max_tokens'`，改纯受控 `InputNumber`，submit 时显式从 inputs state 读取这两个字段
- [x] 13.4 race detector 通过：`go test -race -run "TestGetRandomSatisfiedChannel|TestFilterChannelsByMaxTokens|TestComputeEffectivePriority|TestCacheGetRandomSatisfiedChannel" ./model/... ./service/...`
