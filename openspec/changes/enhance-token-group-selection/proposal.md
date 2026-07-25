## Why

当前所有"创建带分组的实体"表单都把 `group` 字段当作可选且带有默认值，导致两类问题：

1. **API 密钥创建/编辑**：`group` 字段在 schema 里是 `optional`，默认值预填 `default`（或在系统开启自动分组时预填 `auto`）。用户可以在不主动选分组的情况下直接保存，事后才发现该分组对当前用户不可用（请求时被 `middleware/auth.go` 拦截返回 403），把配置错误暴露到了运行期。
2. **管理员批量创建用户**：`batch-create-drawer.tsx` 的分组字段是一个裸 `<Input>` 输入框，placeholder 是 `default`。管理员不知道系统里实际配置了哪些分组，容易拼错或填入已弃用的分组名。

这两类问题本质相同：**分组选择缺乏"显式确认 + 候选枚举"**。本变更将其统一收敛。

## What Changes

1. **API 密钥创建/编辑表单强制选择分组**：去掉 `default`/`auto` 预填默认值，schema 把 `group` 从 `optional` 升级为必填（`min(1)`），未选择时表单无法提交并展示错误提示。
2. **批量创建用户页分组字段升级为下拉选择**：把 `<Input>` 替换为已有的 `ApiKeyGroupCombobox` 风格的下拉，候选项来自管理员可见的系统分组列表（`group ratio` 配置的所有 keys）。
3. **批量创建用户页新增"API 密钥分组"字段（方案 A 扩展）**：当 `create_token=true` 时，新增"API 密钥分组"下拉，让管理员显式指定生成的 token 的分组。后端 `BatchCreateUsersRequest` 新增 `TokenGroup` 字段，创建 token 时优先使用该值（替换现有 `default_use_auto_group` 决定的隐式逻辑）。
4. **i18n 文案补充**：为新增的校验提示与占位文案添加 `en/zh` 翻译键。

非破坏性变更：后端 `BatchCreateUsers` 的 `token_group` 字段为可选（仅在 `create_token=true` 时校验必填），现有调用方不传该字段时 fallback 到原 `default_use_auto_group` 逻辑，保持兼容。

## Capabilities

### New Capabilities

- `token-group-binding`：API 密钥（令牌）创建与编辑表单的分组字段必须由用户显式选择一个有效分组，不再接受空值或预填默认值。
- `batch-user-group-selection`：管理员批量创建用户流程中，**用户分组**与（开启"创建 API 密钥"时的）**API 密钥分组**字段都从自由输入/系统隐式默认升级为下拉枚举选择，候选项来自系统配置的分组集合。

### Modified Capabilities

无（`openspec/specs/` 当前为空，相关能力首次纳入规格管理）。

## Impact

- **前端**：
  - `web/default/src/features/keys/lib/api-key-form.ts`：调整 `getApiKeyFormSchema` 与 `getApiKeyFormDefaultValues`，使 `group` 必填且默认为空。
  - `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx`：去掉基于 `defaultUseAutoGroup` 的预填分支；`ApiKeyGroupCombobox` 已经是下拉，无需更换组件，只需校验生效后展示 `FormMessage`。
  - `web/default/src/features/users/components/batch-create-drawer.tsx`：把 `group` 字段从 `<Input>` 替换为下拉选择组件；新增 `token_group` 字段（仅在 `create_token=true` 时显示），候选项同 `group` 但**包含** `auto`（token 分组允许 auto）。
  - 可能需要把 `ApiKeyGroupCombobox` 提取到更通用的位置或新建一个管理员分组下拉组件（设计阶段决定）。
- **后端**：
  - `controller/user.go`：`BatchCreateUsersRequest` 新增 `TokenGroup string` 字段；`BatchCreateUsers` 处理函数在 `create_token=true` 时校验 `token_group` 非空。
  - `model/user.go`：`BatchCreateUserRequest` 新增 `TokenGroup string` 字段；`BatchCreateUsers` 内创建 token 时用 `req.TokenGroup` 覆盖现有 `default_use_auto_group` 决定的隐式逻辑。
  - 本次无 controller/model 改动**（注：方案 A 扩展后此句作废）**。若现有"管理员可见的所有分组"API 不足以支撑前端下拉（需要确认 `controller/group.go` 的 `/api/group/` 返回内容），可能需要在该接口的响应中补充倍率与描述字段，或在管理端新增 `GET /api/group/list` 端点。具体方案在 `design.md` 决定。
- **i18n**：新增 `Please select a group` 等校验文案的中英文翻译。
- **数据库**：无变更。
- **回归风险**：现有令牌如果 `group` 为空字符串，编辑时会被新校验拦截，需要在编辑回填时做兼容处理（参考 `transformApiKeyToFormDefaults`，把空字符串映射到 `default` 或保留并允许保存）。
