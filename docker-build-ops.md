# Docker 镜像构建与推送操作指南

> 适用场景：将 new-api 项目构建为 Docker 镜像并推送至阿里云容器镜像服务
> 构建环境：macOS + Docker Desktop（Apple Silicon / Intel）

---

## 前置条件

| 项目 | 要求 |
|------|------|
| macOS | Apple Silicon (ARM64) 或 Intel (AMD64) |
| Docker Desktop | v4.x+ 已启动并运行 |
| 阿里云 ACKEY | 已配置镜像仓库访问凭证 |
| Docker Login | 已登录 `registry.cn-hangzhou.aliyuncs.com` |

### 验证 Docker 状态

```bash
docker info | grep -E "Server Version|Operating Version"
```

### 登录阿里云镜像仓库（首次或凭证过期时）

```bash
docker login registry.cn-hangzhou.aliyuncs.com
# 输入用户名和密码（或在阿里云控制台获取临时凭证）
```

---

## 镜像信息

- **仓库地址**: `registry.cn-hangzhou.aliyuncs.com/study_yang/new-api`
- **版本来源**: 项目根目录 `VERSION` 文件
- **推送 tags**: `<版本号>` + `latest`
- **目标平台**: `linux/amd64`
- **构建文件**: `Dockerfile.aliyun`（项目已内置）

---

## 操作步骤

### 1. 确认 VERSION

```bash
cat VERSION
```

### 2. 获取最新代码

```bash
git pull origin main
```

### 3. Dockerfile.aliyun 说明

项目已内置 `Dockerfile.aliyun`，相对 `Dockerfile` 的差异：

1. 基础镜像源从 `docker.io` 改为 `docker.m.daocloud.io`（国内镜像源，避免 docker.io 访问受限）
2. 添加 Go 代理 `GOPROXY=https://goproxy.cn,direct`（加速 Go 模块下载）
3. 版本号从 `VERSION` 文件注入：`-X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'`

```dockerfile
# 修改1: 基础镜像源
FROM docker.m.daocloud.io/oven/bun:1 AS builder           # 原: oven/bun:1
FROM docker.m.daocloud.io/oven/bun:1 AS builder-classic    # 原: oven/bun:1
FROM docker.m.daocloud.io/library/golang:1.26.1-alpine AS builder2  # 原: golang:1.26.1-alpine
FROM docker.m.daocloud.io/library/debian:bookworm-slim      # 原: debian:bookworm-slim

# 修改2: 添加 Go 代理（在 builder2 的 ENV 中增加）
ENV GOPROXY=https://goproxy.cn,direct

# 修改3: 版本号从 VERSION 文件注入
RUN go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api
```

### 4. 构建并推送

#### 4.1 常规构建（带缓存）

```bash
VERSION=$(cat VERSION)

docker buildx build \
  --builder desktop-linux \
  --platform linux/amd64 \
  -t registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:${VERSION} \
  -t registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:latest \
  --push \
  --progress=plain \
  -f Dockerfile.aliyun \
  .
```

> **注意**：`--progress=plain` 可实时显示构建日志，便于排查问题。

#### 4.2 强制无缓存构建（前端代码变更未生效时使用）

当出现 **前端代码改动未反映到镜像中**（例如新增按钮、页面未更新）时，Docker build cache 可能复用了旧的 frontend `dist/`。此时必须使用 `--no-cache` 强制重建：

```bash
VERSION=$(cat VERSION)

docker buildx build \
  --builder desktop-linux \
  --platform linux/amd64 \
  --no-cache \
  -t registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:${VERSION} \
  -t registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:latest \
  --push \
  --progress=plain \
  -f Dockerfile.aliyun \
  .
```

> **何时使用 `--no-cache`**：
> - 前端源码改动后构建，但镜像中仍是旧界面
> - `COPY ./web/default ./default` 显示 `CACHED`，但源码已变
> - 怀疑任何阶段的缓存层与实际代码不同步
>
> **代价**：耗时从 ~3 分钟增加到 ~10-15 分钟（所有阶段重新执行）

### 5. 验证推送结果

```bash
# 查看远程镜像（需要在阿里云控制台确认）
# 或使用 crane 工具
crane digest registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:${VERSION}
```

---

## 常见问题

### Q1: `pull access denied for debian` 或 `authorization failed`

**原因**: Docker Desktop 无法访问 `docker.io`（网络受限）

**解决**: 基础镜像改用 `docker.m.daocloud.io` 国内镜像源

### Q2: `fork/exec /usr/bin/unpigz: exec format error`

**原因**: Docker Desktop Linux VM 内部的 `unpigz` 二进制损坏或架构不匹配

**解决**: 在 Docker Desktop 中点击 "Restart" 或执行：

```bash
osascript -e 'quit app "Docker"' && sleep 5 && open -a Docker
```

等待 Docker 重新启动（约 30-60 秒），所有设置了 `unless-stopped` 策略的容器会自动恢复。

### Q3: `go mod download` 超时 / `proxy.golang.org` 无法访问

**原因**: Go 默认代理 `proxy.golang.org` 在国内不稳定

**解决**: 在 Dockerfile 中添加：

```dockerfile
ENV GOPROXY=https://goproxy.cn,direct
```

### Q4: `bun install` 下载慢或 tarball 提取失败

**原因**: Docker Desktop VM 内访问 npm registry 网络不稳定，导致 tarball 下载中断；特别是 `@esbuild/linux-x64` 等平台特定二进制包

**解决**（推荐方案，已验证有效）:

在 Dockerfile 的两个前端构建阶段（`builder` 和 `builder-classic`）添加 `.npmrc`，将 registry 指向国内镜像：

```dockerfile
# 在 bun install 之前添加
RUN echo 'registry=https://registry.npmmirror.com' > .npmrc \
    && bun install --frozen-lockfile
```

> **关键点**：
> - `bun:1` 镜像内没有 `npm` 命令，必须通过 `.npmrc` 配置 registry（bun 会读取该文件）
> - `npmmirror` 国内镜像可同时加速普通包和平台特定二进制包的下载
> - 经实测，此配置不会触发 bun 的完整性校验失败

**错误示例**:
```
error: Fail extracting tarball for "@esbuild/linux-x64"
```

**其它备选**（如 npmmirror 仍失败）:
- 重启 Docker Desktop VM
- 等待网络恢复后重试（保持 npm 官方源）

### Q5: 如何只构建不推送？

去掉 `--push` 参数，改用 `--load`（仅本机使用）：

```bash
docker buildx build \
  --platform linux/amd64 \
  --load \
  -t new-api:local \
  -f Dockerfile.aliyun \
  .
```

### Q6: 如何验证镜像是否包含最新前端代码？

**原因**: Docker build cache 可能复用旧的 frontend `dist/`，导致镜像内容滞后于源码

**快速验证**（推荐）：直接从镜像中提取二进制文件搜索关键字：

```bash
# 1. 创建临时容器并复制二进制
docker create --name verify <镜像标签>
docker cp verify:/new-api /tmp/verify-new-api
docker rm verify

# 2. 搜索关键字（如新增按钮的文字）
strings /tmp/verify-new-api | grep "关键字"
```

> **说明**: Go 二进制通过 `go:embed` 将 frontend dist 打包进去，`strings` 命令可检出嵌入的 JS chunk 中的文字

### Q7: 镜像中前端功能缺失（缓存导致）

**症状**: 源码已修改但镜像中仍是旧界面，`docker buildx build` 日志显示 `CACHED`

**根因**: Docker layer caching 在 `COPY ./web/default ./default` 阶段命中缓存（该层及后续所有层均使用旧产物）

**解决**:
1. 添加 `--no-cache` 参数强制重建（见 4.2 节）
2. 重新构建并推送
3. 用 Q6 方法验证新镜像包含目标内容

---

## 完整操作清单

```bash
# 1. 确认版本
cat VERSION

# 2. 拉取最新代码
git pull origin main

# 3. 构建并推送（Dockerfile.aliyun 已内置）
VERSION=$(cat VERSION)
docker buildx build \
  --builder desktop-linux \
  --platform linux/amd64 \
  -t registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:${VERSION} \
  -t registry.cn-hangzhou.aliyuncs.com/study_yang/new-api:latest \
  --push \
  --progress=plain \
  -f Dockerfile.aliyun \
  .
```

---

## 构建耗时参考

| 步骤 | 耗时 | 说明 |
|------|------|------|
| 基础镜像拉取 | 10-30s | 首次或缓存失效时 |
| bun install (default) | 120-150s | 1400+ 包，网络 IO 密集 |
| bun install (classic) | 100-130s | 1000+ 包，与 default 并行 |
| bun run build (前端编译) | 90-120s | rsbuild 编译 |
| go build (Go 编译) | 300-350s | 最耗时步骤 |
| 镜像推送 | 30-45s | 取决于网络 |
| **总计** | **约 10-15 分钟** | |

---

## 相关命令速查

| 场景 | 命令 |
|------|------|
| 查看本地镜像 | `docker images` |
| 查看运行中容器 | `docker ps` |
| 查看 Docker 状态 | `docker info` |
| 重启 Docker Desktop | `osascript -e 'quit app "Docker"' && open -a Docker` |
| Buildx 构建日志 | `docker-desktop://dashboard/build/...` |
| 查看阿里云镜像 | 阿里云容器镜像服务控制台 |
| 查看 buildx builder 状态 | `docker buildx inspect desktop-linux` |
| 从镜像提取内容验证 | `docker create --name v <tag> && docker cp v:/new-api /tmp/v && strings /tmp/v \| grep "关键字"` |
