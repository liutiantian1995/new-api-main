# v1.0.0-rc.15-batch4 增量部署文档

> 版本：v1.0.0-rc.15-batch5
> 镜像：`registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch5`


---

## 一、本次变更摘要

| 变更类型 | 内容 | 影响范围 |
|---------|------|---------|
| 新增字段 | `channels.max_tokens`（int, default 0）<br>`channels.token_tiers`（JSON text） | 数据库 |
| 新增服务 | `service/token_estimate.go` — 输入 token 估算 | 后端 |
| 修改算法 | `model/channel_cache.go` `GetRandomSatisfiedChannel` 新增 `estTokens` 参数与 `fallback bool` 返回值 | 后端 |
| 修改中间件 | `middleware/distributor.go` 注入 estTokens、affinity token 守卫、fallback header | 后端 |
| 新增校验 | `controller/channel.go` `validateChannel` 加 `token_tiers` 数量上限 10、`max_tokens>=0`、`priority_boost ∈ [-100, 100]` | 后端 |
| 新增前端面板 | classic 主题 `TokenRoutingPanel.jsx`<br>default 主题 `token-routing-section.tsx` | 前端 |
| 新增响应头 | `X-Token-Routing-Fallback: max-tokens-exceeded` | 后端 |

### 本批次（batch4）相对 batch3 的增量

| 严重级 | 问题 | 修复 |
|--------|------|------|
| 🔴 CRITICAL | 包级 `tokenRoutingFallbackSeen` 在 RLock 内写入，存在数据竞争与跨请求串扰 | 删除包级状态，`GetRandomSatisfiedChannel`/`CacheGetRandomSatisfiedChannel` 多返回 `fallback bool` |
| 🟠 HIGH | 服务端缺 `token_tiers` 数量上限校验，可被恶意 PUT 放大 CPU | `validateChannel` 加 `len(TokenTiers) <= 10`，补单元测试 |
| 🟠 HIGH | Classic 主题 `Form.InputNumber field='max_tokens'` 与 props 双源 | 改纯受控 `InputNumber`，submit 时显式从 inputs state 读取 |

---

## 二、数据库变更

### 2.1 channels 表新增字段

通过 GORM AutoMigrate 自动添加，**不删除任何现有字段**。

**SQLite**：
```sql
ALTER TABLE channels ADD COLUMN max_tokens INTEGER DEFAULT 0;
ALTER TABLE channels ADD COLUMN token_tiers TEXT;
```

**MySQL**：
```sql
ALTER TABLE channels ADD COLUMN max_tokens INT DEFAULT 0;
ALTER TABLE channels ADD COLUMN token_tiers TEXT;
```

**PostgreSQL**：
```sql
ALTER TABLE channels ADD COLUMN max_tokens INTEGER DEFAULT 0;
ALTER TABLE channels ADD COLUMN token_tiers TEXT;
```

### 2.2 字段语义

| 字段 | 类型 | 默认 | 含义 |
|------|------|------|------|
| `max_tokens` | int | 0 | 该渠道能稳定承载的最大 estTokens；超过则软过滤跳过。0=不限 |
| `token_tiers` | JSON text | `[]` | 分档数组，每项 `{max_tokens, priority_boost}` |
| `token_tiers[].max_tokens` | int (>0) | — | 分档阈值，estTokens ≤ 此值时触发对应 boost |
| `token_tiers[].priority_boost` | int [-100, 100] | — | 累加到该渠道 effective_priority |

**示例**：
```json
{
  "max_tokens": 200000,
  "token_tiers": [
    {"max_tokens": 50000,  "priority_boost": 5},
    {"max_tokens": 128000, "priority_boost": 2}
  ]
}
```

### 2.3 迁移方式

项目启动时 GORM AutoMigrate 自动执行，无需手动运行 SQL。

**验证迁移成功**（可选）：
```sql
-- SQLite
PRAGMA table_info(channels);
-- 应包含 max_tokens、token_tiers 两列

-- MySQL
DESCRIBE channels;

-- PostgreSQL
\d channels
```

### 2.4 兼容性保证

- 旧客户端请求/响应不含新字段时，按默认值 `0` / `[]` 持久化（GORM 默认值处理）
- 未配置 token_tiers 的渠道，`effective_priority = base priority`，行为完全同 main 分支
- 三种数据库（SQLite / MySQL / PostgreSQL）均已通过 migration 测试

---

## 三、部署步骤

### 3.1 前置检查

- [ ] 当前运行版本为 `v1.0.0-rc.15` 或兼容版本
- [ ] 数据库连接正常
- [ ] Redis 连接正常（如启用）
- [ ] 已备份 `channels` 表（可选但推荐）

### 3.2 拉取新镜像

```bash
docker pull registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch5
```

### 3.3 更新容器

**方式一：docker-compose**
```yaml
services:
  new-api:
    image: registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch5
```
```bash
docker-compose up -d
```

**方式二：docker run**
```bash
docker stop new-api && docker rm new-api
docker run -d \
  --name new-api \
  --restart always \
  -p 3000:3000 \
  -v ./data:/data \
  -v ./logs:/app/logs \
  -e SQL_DSN="your_dsn" \
  -e REDIS_CONN_STRING="your_redis" \
  -e TZ=Asia/Shanghai \
  registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch5
```

### 3.4 验证部署

```bash
# 1. 检查容器状态
docker ps | grep new-api

# 2. 查看启动日志（确认 AutoMigrate 无报错）
docker logs new-api --tail 50

# 3. 验证 API 可达
curl http://localhost:3000/api/status

# 4. 验证新字段存在（需管理员 token）
curl http://localhost:3000/api/channel/ \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" | jq '.data[0] | {max_tokens, token_tiers}'
# 应输出 {"max_tokens": 0, "token_tiers": null} 而非字段缺失
```

### 3.5 验证镜像包含修复

```bash
# 提取二进制搜索审查修复关键字
docker create --name verify registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch5
docker cp verify:/new-api /tmp/verify-new-api
docker rm verify

# C-1 修复标志：fallback bool 返回值
strings /tmp/verify-new-api | grep -c "token_tiers 数量上限"
# 应输出 ≥ 1
```

---

## 四、功能验证清单

### 4.1 配置 Token 路由策略

1. 登录管理后台 → 渠道管理 → 编辑某个渠道
2. 找到「Token 路由策略」面板（默认收起），展开
3. 填写：
   - `max_tokens`：`200000`
   - 添加分档：`≤ 50000 → +5`、`≤ 128000 → +2`
4. 保存
5. 验证数据库：
   ```sql
   SELECT max_tokens, token_tiers FROM channels WHERE id = <渠道ID>;
   -- 应返回 200000 与 [{"max_tokens":50000,"priority_boost":5},{"max_tokens":128000,"priority_boost":2}]
   ```

### 4.2 路由行为验证

| 测试场景 | 请求 estTokens | 期望行为 | 验证方式 |
|---------|---------------|---------|---------|
| 小请求走 boost 渠道 | ~10K | 走分档 boost 的渠道 | 日志查看 effective_priority |
| 中等请求正常路由 | ~80K | 未超 max_tokens，正常路由 | 响应正常 |
| 大请求软过滤 | ~300K | max_tokens=200K 的渠道被跳过 | 该渠道无新请求记录 |
| 超大请求全空回退 | ~5M | 所有渠道都被过滤 → 回退全集合 | 响应头含 `X-Token-Routing-Fallback: max-tokens-exceeded` |

### 4.3 服务端校验验证

```bash
# 构造 11 个分档（超过上限）
curl -X PUT http://localhost:3000/api/channel/ \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": 1,
    "max_tokens": 200000,
    "token_tiers": [
      {"max_tokens": 10000, "priority_boost": 1},
      {"max_tokens": 20000, "priority_boost": 1},
      {"max_tokens": 30000, "priority_boost": 1},
      {"max_tokens": 40000, "priority_boost": 1},
      {"max_tokens": 50000, "priority_boost": 1},
      {"max_tokens": 60000, "priority_boost": 1},
      {"max_tokens": 70000, "priority_boost": 1},
      {"max_tokens": 80000, "priority_boost": 1},
      {"max_tokens": 90000, "priority_boost": 1},
      {"max_tokens": 100000, "priority_boost": 1},
      {"max_tokens": 110000, "priority_boost": 1}
    ]
  }'
# 应返回 400 错误，message 含 "token_tiers 数量上限为 10"
```

### 4.4 Classic 主题受控组件验证

1. 在 classic 主题编辑渠道页打开「Token 路由策略」
2. 修改 `max_tokens` 输入框
3. **不要点保存**，刷新页面
4. 输入框应恢复为数据库原值（证明纯受控，无内部状态残留）
5. 再次修改并保存，验证值正确持久化

### 4.5 回归验证

- [ ] 未配置 token_tiers 的渠道，路由行为与升级前完全一致
- [ ] MJ / Suno / audio / images / realtime 路径不估算 token，零开销
- [ ] affinity 命中渠道但 estTokens > affinityCh.max_tokens 时，亲和失效重新选 channel

---

## 五、性能指标

| 指标 | 实测 | 目标 |
|------|------|------|
| 100KB body token 估算耗时 | ~0.7ms 中位数 | <1ms |
| 50 候选渠道 effective_priority 计算 | <0.001ms | <0.1ms |
| 非文本路径开销 | 0（跳过估算） | 0 |

---

## 六、回滚方案

### 6.1 镜像回滚

```yaml
# docker-compose 回到 batch3 或 rc.15
services:
  new-api:
    image: registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch5
```
```bash
docker-compose up -d
```

### 6.2 配置回滚（无需回滚镜像）

**清空渠道配置即恢复原行为，无需任何 schema 变更**：

```sql
-- 将所有渠道的 token 路由策略清空
UPDATE channels SET max_tokens = 0, token_tiers = '[]' WHERE max_tokens > 0 OR token_tiers IS NOT NULL;
```

清空后：
- `effective_priority = base priority`
- 所有请求都通过全集合路由，行为完全同 main 分支
- `X-Token-Routing-Fallback` header 永不出现

### 6.3 字段保留建议

新增的 `max_tokens` / `token_tiers` 列建议**保留不删**：
- 删除列需 `ALTER TABLE DROP COLUMN`，SQLite 老版本不支持
- 保留空列对查询性能无影响
- 未来若重新启用，无需再次 migration

---

## 七、注意事项

1. **AutoMigrate 幂等性**：重复启动不会重复添加列或修改结构
2. **估算偏差**：token 估算偏差 ±20%，配置 `max_tokens` 时建议留 15-20% buffer
3. **tier 数量上限**：单渠道最多 10 条 `token_tiers`（服务端 + 客户端双重限制）
4. **priority_boost 范围**：[-100, 100]，避免极端值（>50）导致 tier 跨度过大
5. **fallback header 语义**：`X-Token-Routing-Fallback: max-tokens-exceeded` 表示本次因 estTokens 超过所有候选 `max_tokens` 而走了全集合（诊断信号，非错误）
6. **affinity 守卫**：用户对小请求建立的渠道亲和，遇到大请求时会自动失效，避免错误路由到大容量受限渠道
7. **auto-group 透传**：跨组迭代时使用同一 estTokens（请求未变，无需重算）
8. **零成本默认**：未配置时性能与 main 分支持平，不影响全局路由均衡

---

## 八、tier 边界 buffer 建议

- 上游真实 limit 的 **80%** 作为 `max_tokens`（留 buffer 防止边界失败）
- 分档阈值参考典型请求规模：
  - `{32K, +3}` — 小请求优先
  - `{128K, +2}` — 中等请求适度提权
  - `{200K, -5}` — 大请求降权（负值表示避开高负载渠道）
- 避免极端 `priority_boost`（>50）导致 tier 跨度过大，破坏加权随机均衡

---

## 九、相关代码位置

| 模块 | 文件 | 关键符号 |
|------|------|---------|
| 数据模型 | `model/channel.go:59` | `Channel.MaxTokens`, `Channel.TokenTiers`, `TokenTier` |
| 缓存加载 | `model/channel_cache.go` | `InitChannelCache`, `GetRandomSatisfiedChannel` |
| 软过滤 | `model/channel_cache.go` | `filterChannelsByMaxTokens` |
| 优先级计算 | `model/channel_cache.go` | `computeEffectivePriority` |
| Token 估算 | `service/token_estimate.go` | `EstimateInputTokens` |
| Service 透传 | `service/channel_select.go` | `RetryParam.EstTokens`, `CacheGetRandomSatisfiedChannel` |
| 中间件 | `middleware/distributor.go:176` | `X-Token-Routing-Fallback` header |
| 输入校验 | `controller/channel.go` | `validateChannel` |
| Classic 前端 | `web/classic/src/components/table/channels/modals/TokenRoutingPanel.jsx` | 纯受控组件 |
| Default 前端 | `web/default/src/features/channels/components/drawers/sections/token-routing-section.tsx` | Base UI Accordion |
| i18n | `web/default/src/i18n/locales/{en,zh}.json` | `Token Routing Strategy` 等 key |
