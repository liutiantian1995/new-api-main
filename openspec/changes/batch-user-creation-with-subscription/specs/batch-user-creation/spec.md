## ADDED Requirements

### Requirement: 批量创建用户 API

系统 SHALL 提供 `POST /api/user/batch` 端点，允许管理员在单个请求中批量创建多个用户。该端点 MUST 在单个数据库事务内完成所有用户、订阅、令牌的创建，保证原子性。

#### Scenario: 成功批量创建用户

- **WHEN** 管理员发送 POST `/api/user/batch`，请求体包含 `prefix=test`、`date=0601`、`count=5`、`group=default`、`plan_id=1`、`create_token=true`、`activation_strategy=on_use`
- **THEN** 系统创建 5 个用户，用户名分别为 `test060101`、`test060102`、`test060103`、`test060104`、`test060105`，密码为 `{用户名}@123`，分组为 `default`，并为每个用户创建套餐 1 的 `on_use` 订阅和默认 API 密钥
- **AND** 响应返回成功创建的用户列表（含用户名、ID）

#### Scenario: 用户名前缀为空

- **WHEN** 管理员发送请求，`prefix` 为空或仅含空白字符
- **THEN** 系统返回 400 错误，消息为"用户名前缀不能为空"

#### Scenario: 批量数量超过上限

- **WHEN** 管理员发送请求，`count` 超过 200
- **THEN** 系统返回 400 错误，消息为"单次批量创建数量不能超过 200"

#### Scenario: 用户名冲突

- **WHEN** 批量创建的用户名中存在与数据库已有用户名冲突的情况
- **THEN** 整个事务回滚，系统返回 400 错误，消息包含冲突的用户名

#### Scenario: 非管理员调用

- **WHEN** 非管理员用户（role < RoleAdminUser）调用该端点
- **THEN** 系统返回 403 错误

### Requirement: 用户名生成规则

系统 SHALL 按照 `{prefix}{date}{seq}` 格式生成批量用户名。`seq` MUST 根据批量数量补零至合适位数（如 count=50 则 2 位，count=200 则 3 位）。密码 MUST 为 `{用户名}@123`。

#### Scenario: 序号补零

- **WHEN** 管理员发送请求，`prefix=user`、`date=0601`、`count=50`
- **THEN** 生成的用户名为 `user060101` 到 `user060150`（2 位序号补零）

#### Scenario: 日期为空

- **WHEN** 管理员发送请求，`prefix=user`、`date` 为空字符串、`count=3`
- **THEN** 生成的用户名为 `user001` 到 `user003`（日期部分省略）

### Requirement: 批量设置分组

系统 SHALL 支持在批量创建时为所有用户设置相同的分组。

#### Scenario: 指定有效分组

- **WHEN** 管理员发送请求，`group=vip`
- **THEN** 所有批量创建的用户的 `group` 字段为 `vip`

#### Scenario: 未指定分组

- **WHEN** 管理员发送请求，`group` 为空
- **THEN** 所有批量创建的用户的 `group` 字段为系统默认值 `default`

### Requirement: 批量创建 API 密钥

系统 SHALL 支持在批量创建时为每个用户创建默认 API 密钥。

#### Scenario: 启用密钥创建

- **WHEN** 管理员发送请求，`create_token=true`
- **THEN** 为每个用户创建一个 API 密钥，密钥名称为 `{用户名}的初始令牌`，永不过期，无限额度，分组为 `auto`（若系统启用 DefaultUseAutoGroup）

#### Scenario: 禁用密钥创建

- **WHEN** 管理员发送请求，`create_token=false` 或未指定
- **THEN** 不为用户创建 API 密钥

### Requirement: 批量创建审计日志

系统 SHALL 在批量创建成功后记录管理审计日志。

#### Scenario: 审计日志记录

- **WHEN** 批量创建 5 个用户成功
- **THEN** 系统记录 1 条 `user.batch_create` 审计日志，包含创建数量、前缀、分组、套餐 ID、生效策略
