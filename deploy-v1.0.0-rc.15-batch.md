# v1.0.0-rc.15-batch 增量部署文档

> 版本：v1.0.0-rc.15-batch
> 镜像：`registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch`
> 基础版本：v1.0.0-rc.15

---

## 一、本次变更摘要

| 变更类型 | 内容 | 影响范围 |
|---------|------|---------|
| 新增表 | `pending_subscription_activations` | 数据库 |
| 新增 API | `POST /api/user/batch` | 后端 |
| 存量接口扩展 | `POST /subscription/admin/users/:id/subscriptions`、`POST /subscription/admin/bind` 增加可选参数 `activation_strategy` | 后端 |
| 新增前端页面 | 批量创建用户抽屉 | 前端 |
| 存量前端扩展 | 用户订阅管理弹窗增加"生效策略"下拉选项 | 前端 |

---

## 二、数据库变更

### 2.1 新增表 `pending_subscription_activations`

通过 GORM AutoMigrate 自动创建，**不修改任何现有表**。

```sql
CREATE TABLE pending_subscription_activations (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id              INTEGER NOT NULL,
    user_subscription_id INTEGER NOT NULL,
    plan_id              INTEGER NOT NULL,
    activation_strategy  VARCHAR(16) NOT NULL DEFAULT 'on_use',
    status               VARCHAR(16) NOT NULL DEFAULT 'pending',
    activated_at         BIGINT NOT NULL DEFAULT 0,
    created_at           BIGINT NOT NULL,
    updated_at           BIGINT NOT NULL
);

CREATE INDEX idx_pending_subscription_activations_user_id ON pending_subscription_activations(user_id);
CREATE INDEX idx_pending_subscription_activations_user_subscription_id ON pending_subscription_activations(user_subscription_id);
CREATE INDEX idx_pending_subscription_activations_plan_id ON pending_subscription_activations(plan_id);
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_id` | INT, INDEX | 关联用户 ID |
| `user_subscription_id` | INT, INDEX | 关联订阅 ID（`user_subscriptions.id`） |
| `plan_id` | INT, INDEX | 关联套餐 ID |
| `activation_strategy` | VARCHAR(16) | 生效策略，固定为 `on_use` |
| `status` | VARCHAR(16) | `pending`（待激活）/ `activated`（已激活） |
| `activated_at` | BIGINT | 实际激活时间戳，0 表示未激活 |
| `created_at` | BIGINT | 创建时间戳 |
| `updated_at` | BIGINT | 更新时间戳 |

### 2.2 现有表变更

**无**。`user_subscriptions` 表零修改，仅复用 `Status` 字段的 `pending` 取值（原有取值为 `active`/`expired`/`cancelled`）。

### 2.3 迁移方式

项目启动时 GORM AutoMigrate 自动执行，无需手动运行 SQL。

**验证迁移成功**（可选）：

```sql
-- SQLite
.tables
-- 应包含 pending_subscription_activations

-- MySQL
SHOW TABLES LIKE 'pending_subscription_activations';

-- PostgreSQL
SELECT tablename FROM pg_tables WHERE tablename = 'pending_subscription_activations';
```

---

## 三、部署步骤

### 3.1 前置检查

- [ ] 当前运行版本为 `v1.0.0-rc.15` 或兼容版本
- [ ] 数据库连接正常
- [ ] Redis 连接正常（如启用）

### 3.2 拉取新镜像

```bash
docker pull registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch
```

### 3.3 更新容器

**方式一：docker-compose**

```yaml
# docker-compose.yml 修改 image
services:
  new-api:
    image: registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch
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
  registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15-batch
```

### 3.4 验证部署

```bash
# 1. 检查容器状态
docker ps | grep new-api

# 2. 查看启动日志（确认 AutoMigrate 无报错）
docker logs new-api --tail 50

# 3. 验证 API 可达
curl http://localhost:3000/api/status

# 4. 验证新接口存在（需管理员 token）
curl -X POST http://localhost:3000/api/user/batch \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"prefix":"test","count":1}'
# 应返回 {"success":true,...} 而非 404
```

---

## 四、功能验证清单

### 4.1 批量创建用户

1. 登录管理后台 → 用户管理
2. 点击"批量创建"按钮
3. 填写表单：
   - 用户名前缀：`test`
   - 日期后缀：`0601`
   - 数量：`3`
   - 分组：`default`
   - 订阅套餐：选择任意套餐
   - 生效策略：`使用时生效`
   - 创建 API 密钥：勾选
4. 点击"批量创建"
5. 验证：
   - 用户列表出现 `test06011`、`test06012`、`test06013`
   - 每个用户有 1 条 `pending` 状态的订阅（在用户订阅管理中查看）
   - 每个用户有 1 个 API 密钥

### 4.2 使用时激活订阅

1. 使用批量创建的用户登录（密码为 `用户名@123`）
2. 登录成功后，检查数据库：
   ```sql
   SELECT * FROM pending_subscription_activations WHERE user_id = <用户ID>;
   -- status 应从 pending 变为 activated，activated_at 应有值
   ```
3. 或使用该用户的 API 密钥发起一次 API 请求
4. 再次检查订阅状态：
   ```sql
   SELECT * FROM user_subscriptions WHERE user_id = <用户ID>;
   -- status 应从 pending 变为 active，start_time/end_time 应有值
   ```

### 4.3 存量接口兼容性

1. 通过"用户管理 → 用户订阅管理"为已有用户添加订阅（不选生效策略）
2. 验证订阅立即生效（`status=active`，`start_time` 有值）
3. 验证 `pending_subscription_activations` 表中**无**对应记录

---

## 五、回滚方案

### 5.1 回滚到 v1.0.0-rc.15

```bash
# docker-compose
services:
  new-api:
    image: registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0-rc.15

docker-compose up -d
```

### 5.2 数据库回滚（如需要）

```sql
-- 删除新增表（数据丢失，谨慎操作）
DROP TABLE IF EXISTS pending_subscription_activations;

-- 清理 user_subscriptions 中 status='pending' 的记录（如存在）
DELETE FROM user_subscriptions WHERE status = 'pending';
```

---

## 六、注意事项

1. **AutoMigrate 幂等性**：重复启动不会重复创建表或修改表结构
2. **`pending` 状态订阅不影响计费**：现有计费逻辑查询条件为 `status='active' AND end_time > now`，`pending` 状态被自动过滤
3. **激活触发时机**：用户首次登录（任意方式）或首次使用 API 密钥调用 API 时自动激活，无需手动操作
4. **密码规则**：批量创建的用户初始密码为 `用户名@123`，建议通知用户首次登录后修改密码
5. **批量上限**：单次最多创建 200 个用户，超过需分批操作
