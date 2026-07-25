## ADDED Requirements

### Requirement: 批量创建用户表单分组字段升级为下拉选择

管理员批量创建用户表单的 `group` 字段 MUST 渲染为下拉选择组件（不再是自由输入框 `<Input>`），候选项来自新增的 `GET /api/group/details` 端点，包含分组名、倍率与描述。

#### Scenario: 管理员看到所有系统分组的下拉

- **WHEN** 管理员打开"批量创建用户"抽屉，焦点进入分组字段
- **THEN** 下拉展示 `ratio_setting.GetGroupRatioCopy()` 返回的所有分组（例如 `default`、`vip`），每个选项展示名称、描述与倍率 badge

#### Scenario: 管理员可在下拉中搜索分组

- **WHEN** 管理员在下拉搜索框输入 `vip`
- **THEN** 选项列表过滤为 label/desc/ratio 包含 `vip` 的子集

### Requirement: 批量创建用户表单分组字段强制必填

`batchFormSchema.group` MUST 用 `z.string().min(1, ...)` 校验阻止空值提交；默认值 MUST 为空字符串，不再依赖 placeholder 隐式默认。

#### Scenario: 未选择分组时不可提交

- **WHEN** 管理员未在分组字段选择任何值，直接点击"批量创建"
- **THEN** 表单校验失败，分组字段下方显示错误提示，POST `/api/user/batch` 请求 NOT 发出

#### Scenario: 选择分组后正常提交

- **WHEN** 管理员从下拉中选择 `default` 分组后点击"批量创建"
- **THEN** POST `/api/user/batch` 请求体中 `group` 字段为 `"default"`，与现有后端 `BatchCreateUsersRequest.Group` 兼容

### Requirement: 新增 GET /api/group/details 端点

后端 MUST 在 `controller/group.go` 新增 `GetGroupDetails` 处理函数，挂载到现有 `groupRoute`（`/api/group/details`，受 `AdminAuth` 保护），返回系统所有分组的完整结构。

#### Scenario: 管理员请求分组详情

- **WHEN** 管理员（携带 admin token）发起 `GET /api/group/details`
- **THEN** 响应体为：
  ```json
  {
    "success": true,
    "data": [
      {"name": "default", "ratio": 1, "desc": "默认分组"},
      {"name": "vip", "ratio": 0.5, "desc": "VIP 分组"}
    ]
  }
  ```
  数据来源是 `ratio_setting.GetGroupRatioCopy()`，按分组名排序以保证响应稳定。

#### Scenario: 非管理员请求被拒绝

- **WHEN** 普通用户或未登录用户发起 `GET /api/group/details`
- **THEN** 返回 401/403（由现有 `AdminAuth` 中间件统一处理），响应体不含分组数据

#### Scenario: 现有 GET /api/group/ 不受影响

- **WHEN** 任意现有调用方请求 `GET /api/group/`
- **THEN** 响应仍为 `{"success": true, "data": ["default", "vip", ...]}`（字符串数组），保持向后兼容

### Requirement: 批量创建页过滤掉 auto 虚拟分组

由于 `auto` 是令牌分组语义，不是用户分组语义，批量创建用户表单的分组下拉 MUST NOT 展示 `auto` 选项。

#### Scenario: auto 不出现在批量创建分组下拉

- **WHEN** 管理员打开"批量创建用户"抽屉查看分组下拉
- **THEN** 选项列表不包含 `auto`，即使系统配置中存在 `auto` 虚拟分组

### Requirement: GroupCombobox 组件提取为共享组件

现有 `ApiKeyGroupCombobox` MUST 被提取到 `web/default/src/components/group-combobox.tsx`，重命名为 `GroupCombobox`，原 `features/keys/components/api-key-group-combobox.tsx` MUST 保留为 re-export shim 以维持向后兼容。

#### Scenario: 现有 import 不被破坏

- **WHEN** `api-keys-mutate-drawer.tsx` 仍执行 `import { ApiKeyGroupCombobox } from '../components/api-key-group-combobox'`
- **THEN** 编译通过，运行时行为与提取前一致（dropdown 渲染、搜索、选择回填均正常）

#### Scenario: 新调用方使用共享组件

- **WHEN** `batch-create-drawer.tsx` 引入 `GroupCombobox` 并传入 `options` 与 `value`
- **THEN** 组件渲染、交互与 `ApiKeyGroupCombobox` 视觉一致

### Requirement: 批量创建用户表单新增 API 密钥分组字段（方案 A 扩展）

当 `create_token=true` 时，表单 MUST 额外展示一个"API 密钥分组"下拉字段，允许管理员显式指定生成的 token 的分组。后端 `BatchCreateUsersRequest` MUST 新增 `TokenGroup` 字段并在 `create_token=true` 时强制非空校验。

#### Scenario: 勾选创建 API 密钥后显示 token 分组字段

- **WHEN** 管理员打开"批量创建用户"抽屉，把"创建 API 密钥"开关切到 ON
- **THEN** 表单新增一个"API 密钥分组"下拉，候选项来自 `GET /api/group/details`（**包含** `auto`），与用户分组下拉并列

#### Scenario: 关闭创建 API 密钥时隐藏 token 分组字段

- **WHEN** 管理员把"创建 API 密钥"开关切到 OFF
- **THEN** "API 密钥分组"字段从表单中移除，提交时也不再校验

#### Scenario: 未选 token 分组时不可提交

- **WHEN** 管理员勾选"创建 API 密钥"但未选 API 密钥分组，点击"批量创建"
- **THEN** 表单校验失败，API 密钥分组字段下方显示错误提示，POST `/api/user/batch` 请求 NOT 发出

#### Scenario: 提交时 token_group 字段被正确携带

- **WHEN** 管理员选择了用户分组 `default` 与 API 密钥分组 `auto`，点击"批量创建"
- **THEN** POST `/api/user/batch` 请求体同时包含 `group:"default"` 与 `token_group:"auto"`，生成的每个用户及其 token 分组与表单选择一致

#### Scenario: token 分组下拉包含 auto 选项

- **WHEN** 管理员展开 API 密钥分组下拉
- **THEN** 选项列表包含 `auto` 虚拟分组（与用户分组下拉不同，后者过滤掉 auto）

### Requirement: 后端 BatchCreateUsersRequest 新增 TokenGroup 字段

后端 controller 的 `BatchCreateUsersRequest` 与 model 的 `BatchCreateUserRequest` MUST 新增 `TokenGroup string` 字段。`BatchCreateUsers` 创建 token 时 MUST 优先使用 `req.TokenGroup`；若调用方未传该字段，则保留原 `default_use_auto_group` 决定的隐式行为，保证向后兼容。

#### Scenario: Controller 校验 create_token=true 时 token_group 必填

- **WHEN** 客户端 POST `/api/user/batch`，请求体 `create_token=true` 且 `token_group` 为空字符串
- **THEN** 返回 400 错误，错误信息提示"创建 API 密钥时必须指定分组"

#### Scenario: 调用方未传 token_group 时保留原行为

- **WHEN** 客户端 POST `/api/user/batch`，请求体不含 `token_group` 字段（旧版客户端），`create_token=true`
- **THEN** 后端按原逻辑决定 token 分组（`default_use_auto_group` 为 true 时使用 `auto`，否则空字符串），与升级前行为一致

#### Scenario: create_token=false 时 token_group 被忽略

- **WHEN** 客户端 POST `/api/user/batch`，请求体 `create_token=false` 且 `token_group="vip"`
- **THEN** 请求通过校验（因为不创建 token），`token_group` 字段被忽略，响应正常
