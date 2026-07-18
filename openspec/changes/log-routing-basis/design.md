## Context

Token-aware channel routing（v1.0.0-rc.15-batch4）已上线，引入了 max_tokens 软过滤、token_tiers boost、effective_priority 分 tier 等机制。但路由决策只通过 `logger.LogInfo` 输出到容器 stdout，不落库。管理员排障时需要：

1. 翻容器日志文件，无法按用户/模型/时间筛选
2. 无法关联请求结果（成功/失败/quota）与路由决策
3. 无法统计路由策略的整体命中率（tier_boost 生效率、fallback 触发率）

现有 `Log` 表有 `Other` JSON 字段，已有 `formatUserLogs` 为非管理员剥离 `admin_info`/`audit_info` 等敏感字段的成熟模式。本次复用该机制。

## Goals / Non-Goals

**Goals:**
- 每条 consume log 记录路由决策依据（basis 类型、est_tokens、base/effective priority、boost、fallback）
- 仅管理员可在使用日志中查看路由依据，非管理员 API 响应完全不包含该字段
- 零 DB schema 变更（复用 `Log.Other` JSON 字段）
- 零性能损耗（routing basis 在 distributor 中已计算，仅多一次 context set + JSON merge）
- classic + default 双主题前端均支持

**Non-Goals:**
- 不改变路由算法本身（仅记录，不干预决策）
- 不新增 DB 列或 migration
- 不做实时路由监控大盘（后续可基于该数据扩展）
- 不记录完整候选集评估过程（仅记录最终选中的决策依据）
- 不为非文本路径（MJ/Suno/images）记录路由依据（estTokens=0 时 basis=default 即可）

## Decisions

### D1: 复用 `Log.Other` JSON 字段 vs 新增列

**选择：复用 `Log.Other`**

- `Other` 已是 JSON string，已有 `admin_info`/`audit_info`/`is_model_mapped` 等结构化 key
- 新增列需 migration + 三种 DB 兼容测试，成本高
- `routing_info` 是诊断元数据，不需索引查询，JSON 完全够用
- 零回滚成本：清空 `routing_info` key 即可，无需 DROP COLUMN

**备选（否决）：** 新增 `routing_basis` 列 — 需 migration，且 `Log` 表行数大，ALTER TABLE 锁表风险

### D2: routing basis 4 种分类的判定逻辑

| basis | 判定条件 | 含义 |
|-------|---------|------|
| `affinity` | 亲和命中分支选中渠道（distributor.go:128/135） | 用户上次渠道亲和，未走加权随机 |
| `tier_boost` | estTokens>0 且 effective_priority > base_priority | token 分档 boost 生效，改变了 tier 排序 |
| `default` | estTokens=0 或 effective_priority == base_priority | 标准 base priority + weight 加权随机 |
| `fallback` | fallback == true（全空回退） | 所有候选被 max_tokens 过滤，走了全集合 |

**判定顺序：** fallback 优先级最高（即使 boost 生效但走了回退，也标记 fallback）；其次 affinity；其次 tier_boost；最后 default。

### D3: context 传递 vs 函数参数

**选择：gin context 传递**

- `RecordConsumeLog` 已接收 `c *gin.Context`，读取 context 零签名变更
- distributor 已有 `common.SetContextKey` 模式，一致性好
- 不污染 `RecordConsumeLogParams` 结构（该 struct 已有 12 个字段）

**备选（否决）：** 在 `RecordConsumeLogParams` 新增 `RoutingBasis` 字段 — 需改所有 caller，且部分 caller（如 task billing）不经过 distributor，无路由信息

### D4: admin-only 剥离复用 `formatUserLogs`

复用 `model/log.go:91-110` 现有模式，在 `formatUserLogs` 的 delete 列表新增 `routing_info`：

```go
delete(otherMap, "routing_info")  // 新增
```

`formatUserLogs` 被 `GetLogByTokenId`、`GetUserLogs` 等面向普通用户的查询调用。管理员路径（`GetAllLogs`）不调用 `formatUserLogs`，保留完整 `Other`。

### D5: routing_info JSON 结构

```json
{
  "routing_info": {
    "basis": "tier_boost",
    "est_tokens": 1000,
    "base_priority": 10,
    "effective_priority": 15,
    "boost": 5,
    "fallback": false
  }
}
```

字段全部用基础类型（string/int/bool），便于前端解析与 Excel 导出。`boost` = effective_priority - base_priority，可能为负（负 boost 降权场景）。

## Risks / Trade-offs

- **[Other 字段膨胀]** `routing_info` 约 120 字节/条，按日均 100w 请求计 ~120MB/天 → 可接受（Log 表本身已含 content/other 大字段）
- **[旧日志无 routing_info]** 前端渲染时 `null`/空处理，显示「-」或「未记录」 → 向后兼容
- **[非文本路径 basis 恒为 default]** MJ/Suno/images 等路径 estTokens=0，basis=default → 符合预期，无需特殊处理
- **[affinity 路径无 effective_priority 计算]** 亲和命中时 channel 直接选中，未走 `computeEffectivePriority` → 记录 base_priority 即可，boost=0，basis=affinity
