# new-api v1.0.0.mine 部署文档

> 镜像基于 `feat/user-rolling-rate-limit` 分支构建，相对 main 新增每用户滚动限流 + Reports 页面。
> 构建时间：2026-07-18

---

## 1. 镜像信息

| 项 | 值 |
|---|---|
| 仓库 | `registry.cn-hangzhou.aliyuncs.com/study_yang/new-api` |
| Tag | `v1.0.0.mine`（或 `latest`） |
| Digest | `sha256:f72ab07acd9630a656df38387830acff84363ea46071659d1e3c1878e6e3b1ba` |
| 平台 | `linux/amd64` |
| 暴露端口 | `3000` |

---

## 2. 前置条件

- 已登录阿里云镜像仓库：`docker login registry.cn-hangzhou.aliyuncs.com`
- 运行环境：Docker 20.10+ 或 K8s
- 数据库：SQLite / MySQL ≥ 5.7.8 / PostgreSQL ≥ 9.6 任一
- Redis：可选（推荐，用于用户限流缓存）

---

## 3. 部署步骤

### 3.1 Docker Run

```bash
docker pull registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0.mine

docker run -d \
  --name new-api \
  -p 3000:3000 \
  -v new-api-data:/data \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0.mine
```

### 3.2 Docker Compose

```yaml
services:
  new-api:
    image: registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:v1.0.0.mine
    container_name: new-api
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - TZ=Asia/Shanghai
      # - SQL_DSN=root:pass@tcp(mysql:3306)/new-api
      # - REDIS_CONN_STRING=redis://redis:6379
    volumes:
      - new-api-data:/data

volumes:
  new-api-data:
```

### 3.3 验证

```bash
# 容器健康
docker ps | grep new-api
docker logs -f new-api

# API 探活
curl http://localhost:3000/api/status
```

---

## 4. 数据库变更（自动迁移，无需手动 SQL）

启动时 GORM `AutoMigrate` 自动执行，支持 SQLite / MySQL / PostgreSQL。

| 表 | 变更 | 说明 |
|---|---|---|
| `users` | ADD COLUMN `rolling_rate_limit VARCHAR(2048)` | 每用户滚动限流配置（JSON） |

**option 表** 会自动新增两个 key（KV 结构，无 schema 变更）：
- `UserRollingRateLimitEnabled` = `false`
- `UserRollingRateLimitGroup` = 默认 tier 数组 JSON

**回滚安全**：新增列向后兼容，回滚到旧版本不会丢数据，旧代码会忽略该列。

---

## 5. 新功能简介

### 5.1 每用户滚动限流（Rolling Rate Limit）

- 长窗口配额（如 1 小时 / 1 天），基于内存 + Redis 持久化
- 两级 tier：`group` 级默认 + `user` 级覆盖
- 配置入口：管理后台 → 系统设置 → User Rolling Rate Limit
- 单用户覆盖：用户管理 → 编辑用户 → `rolling_rate_limit` 字段

### 5.2 Admin Reports 页面

- 路径：管理后台 → Reports（侧边栏新增）
- 维度：Token breakdown（input / cache / output / total）
- Top Channels / Top Users 排行

### 5.3 Dashboard 优化

- 总 Token 数拆分为 Input / Cache / Output 三张卡片

---

## 6. 回滚方案

```bash
# 1. 停止新版本
docker stop new-api && docker rm new-api

# 2. 启动上一版本（按原 tag 替换）
docker run -d --name new-api ... registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:<旧版本>

# 3. 数据库无需操作（rolling_rate_limit 列保留，旧代码忽略）
```

---

## 7. 故障排查

| 现象 | 排查 |
|---|---|
| 启动卡在 `migrate database` | 检查 DB 连接 / 权限，查看 `docker logs new-api` |
| `users` 表未新增列 | 确认未禁用 AutoMigrate；手动 `ALTER TABLE users ADD COLUMN rolling_rate_limit VARCHAR(2048);` |
| 限流不生效 | 后台确认 `UserRollingRateLimitEnabled=true`；用户级覆盖优先于 group |
| Redis 连接失败 | 容器会降级到内存限流（单实例可用，多实例不同步） |
