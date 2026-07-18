## 1. 后端基础设施

- [x] 1.1 在 `constant/context_key.go` 新增 `ContextKeyRoutingBasis` context key
- [x] 1.2 在 `model/` 或 `dto/` 新增 `RoutingInfo` struct：`Basis string`、`EstTokens int`、`BasePriority int64`、`EffectivePriority int64`、`Boost int64`、`Fallback bool`，含 JSON tag
- [x] 1.3 新增 `computeRoutingBasis` 辅助函数，实现 basis 判定优先级：fallback > affinity > tier_boost > default

## 2. 后端 distributor 计算 routing basis

- [x] 2.1 在 `middleware/distributor.go` 亲和命中分支（line 128/135）标记 `affinityUsed = true`
- [x] 2.2 在选渠道完成后（line 178 之后），根据 affinityUsed / estTokens / effective_priority / fallback 计算 `RoutingInfo`
- [x] 2.3 将 `RoutingInfo` 通过 `common.SetContextKey(c, constant.ContextKeyRoutingBasis, routingInfo)` 存入 context
- [x] 2.4 effective_priority 计算：affinity 路径下 = base_priority；其余路径调用 `computeEffectivePriority(channel, estTokens)`

## 3. 后端 log 记录与剥离

- [x] 3.1 在 `model/log.go` `RecordConsumeLog` 中，从 `c` context 读取 `RoutingInfo`，若非 nil 则写入 `params.Other["routing_info"]`
- [x] 3.2 在 `model/log.go` `formatUserLogs` 的 delete 列表新增 `delete(otherMap, "routing_info")`
- [x] 3.3 确认 `RecordTaskBillingLog` 等非 distributor 路径：无 routing_info 时 `params.Other` 不含该 key，无需特殊处理

## 4. 后端单元测试

- [x] 4.1 测试 `computeRoutingBasis`：fallback 优先于 affinity 优先于 tier_boost 优先于 default
- [x] 4.2 测试 `formatUserLogs` 剥离 `routing_info`：非管理员日志 Other 中不含该 key
- [x] 4.3 测试旧日志兼容：Other 中无 routing_info key 时 `formatUserLogs` 不报错
- [x] 4.4 测试 `RecordConsumeLog` 注入 routing_info：context 有值时写入 Other，无值时不写入

## 5. 前端 default 主题

- [x] 5.1 找到 default 主题日志表列定义文件（`web/default/src/features/usage-logs/` 或 `web/default/src/routes/console/log.tsx`）
- [x] 5.2 新增「路由依据」列，列定义中根据用户角色（管理员）条件渲染
- [x] 5.3 实现 basis 类型 → 可读标签映射：`tier_boost`→「分档提权」、`default`→「默认」、`fallback`→「回退」、`affinity`→「亲和」
- [x] 5.4 实现 hover/展开显示详细 routing_info（est_tokens、base/effective priority、boost）
- [x] 5.5 旧日志（routing_info 为空）显示「-」或「未记录」

## 6. 前端 classic 主题

- [x] 6.1 找到 classic 主题日志表列定义文件（`web/classic/src/components/table/logs/`）
- [x] 6.2 新增「路由依据」列，仅管理员可见（列定义条件渲染）
- [x] 6.3 同 default 实现 basis 可读标签映射与详细信息展示
- [x] 6.4 旧日志显示「-」占位

## 7. i18n

- [x] 7.1 在 `web/default/src/i18n/locales/en.json` 与 `zh.json` 新增 key：`Routing Basis`、`Tier Boost`、`Default`、`Fallback`、`Affinity`、`Not Recorded` 等
- [x] 7.2 classic 主题 i18n 文案同步
- [x] 7.3 运行 `bun run i18n:sync` 校验 key 完整性

## 8. 集成验证

- [ ] 8.1 管理员发送文本请求 → 查日志 → `other.routing_info` 含完整决策依据
- [ ] 8.2 非管理员发送文本请求 → 查日志 → `other` 不含 `routing_info` key
- [ ] 8.3 管理员发送 MJ/images 请求 → 查日志 → `routing_info.basis = "default"`，`est_tokens = 0`
- [ ] 8.4 配置 max_tokens 后发送超大请求 → 查日志 → `routing_info.basis = "fallback"`，`fallback = true`
- [ ] 8.5 旧日志（升级前数据）在管理员视图显示「未记录」，不报错
- [ ] 8.6 前端 classic + default 主题均正确显示「路由依据」列，非管理员看不到该列

## 9. 回归与构建

- [ ] 9.1 `go build ./...` 编译通过
- [ ] 9.2 `go test -race ./model/... ./middleware/...` 全部通过
- [ ] 9.3 前端 `bun run build`（default + classic）通过
- [ ] 9.4 验证 `Log.Other` 字段大小未异常膨胀（抽样检查 JSON 长度）
