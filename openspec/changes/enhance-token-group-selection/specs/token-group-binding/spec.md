## ADDED Requirements

### Requirement: API 密钥创建表单分组字段强制必填

API 密钥创建表单的 `group` 字段 MUST 由用户从下拉中选择一个有效分组，schema MUST 用 `min(1)` 校验阻止空值提交，默认值 MUST 为空字符串（不再预填 `default` 或 `auto`）。

#### Scenario: 未选择分组时表单不可提交

- **WHEN** 用户打开"创建 API 密钥"抽屉，未在分组字段选择任何值，直接点击"保存"
- **THEN** 表单校验失败，分组字段下方显示 `Please select a group` 错误提示，提交请求 NOT 发出

#### Scenario: 选择了分组后可正常提交

- **WHEN** 用户从分组下拉中选择一个有效分组（例如 `default` 或 `vip`）后点击"保存"
- **THEN** 表单校验通过，POST `/api/token/` 请求携带 `group` 字段为所选值

#### Scenario: 系统开启 default_use_auto_group 时不再预填 auto

- **WHEN** 系统配置 `default_use_auto_group === true`，用户打开"创建 API 密钥"抽屉
- **THEN** 分组字段仍然为空（placeholder 显示 `Select a group`），需要用户主动选择 `auto` 或其他分组

### Requirement: API 密钥编辑表单兼容历史空分组数据

当编辑一个已存在的、`group` 字段为空字符串或 `null` 的令牌时，表单 MUST 在回填后显示分组为空并触发校验提示，引导用户补填分组后再保存。

#### Scenario: 编辑历史空分组令牌时被校验拦截

- **WHEN** 用户打开一个 `group === ''` 或 `group === null` 的现有令牌的编辑抽屉
- **THEN** 分组字段显示为空（placeholder 可见），保存时被 `Please select a group` 校验拦截，直到用户选择一个分组

#### Scenario: 编辑已有分组的令牌时正常显示

- **WHEN** 用户打开一个 `group === 'vip'` 的现有令牌的编辑抽屉
- **THEN** 分组下拉显示 `vip`（含其 ratio badge），保存时校验通过

### Requirement: 分组下拉候选项来自当前用户可用分组

API 密钥表单的分组下拉候选项 MUST 来自 `GET /api/user/self/groups`（登录用户的可用分组集合），包含 `auto` 虚拟分组（若可用），并展示每个分组的倍率与描述。

#### Scenario: 普通用户看不到无权访问的分组

- **WHEN** 普通用户（user.Group = `default`）打开"创建 API 密钥"抽屉
- **THEN** 分组下拉只展示 `service.GetUserUsableGroups('default')` 返回的分组（例如 `default`、`auto`），不展示 `vip` 等未授权分组

#### Scenario: 分组下拉展示倍率与描述

- **WHEN** 用户点击分组下拉展开选项列表
- **THEN** 每个选项展示分组名、描述文本、倍率 badge（例如 `1x Ratio`），与 `ApiKeyGroupCombobox` 现有行为一致
