## Context

当前涉及"分组"的两类创建表单都不够明确：

- **API 密钥表单**（`web/default/src/features/keys/components/api-keys-mutate-drawer.tsx` + `lib/api-key-form.ts`）：`group` 字段是 `z.string().optional()`，默认值预填 `default` 或（当系统开启 `default_use_auto_group` 时）`auto`。用户可以在未主动选择的情况下直接保存，事后调用令牌时才被 `middleware/auth.go` 拦截返回 403。
- **批量创建用户表单**（`web/default/src/features/users/components/batch-create-drawer.tsx`）：`group` 字段是裸 `<Input>`，placeholder 为 `default`。管理员看不到系统实际配置了哪些分组。

后端现有分组相关接口：
- `GET /api/group/`（管理员）→ `controller/group.go:14` `GetGroups`：返回 `[]string`，仅分组名。
- `GET /api/user/groups` 与 `GET /api/user/self/groups`（登录态）→ `controller/group.go:26` `GetUserGroups`：返回 `map[string]{ratio, desc}`，被 `service.GetUserUsableGroups(userGroup)` 过滤，**管理员视角下也可能漏分组**（取决于管理员的 user.Group）。

约束：
- 项目要求跨 SQLite/MySQL/PostgreSQL 兼容，但本次无数据库改动。
- 前端遵循 i18next（英文为 key），新增文案需同步 `zh/en/fr/ru/ja/vi`（其中 fr/ru/ja/vi 走 fallback）。
- `ApiKeyGroupCombobox` 组件已经具备下拉 + 搜索 + 倍率 badge 的能力，可复用。

## Goals / Non-Goals

**Goals:**

1. API 密钥创建/编辑时，分组字段必须由用户显式选择一个有效分组，否则表单不可提交。
2. 批量创建用户表单中，分组字段升级为下拉，候选项覆盖系统配置的所有分组。
3. 现有数据（`group` 为空或为旧值的令牌/用户）在编辑时得到合理兼容，不被新校验锁死。
4. 复用现有 `ApiKeyGroupCombobox` 组件，避免重复实现下拉。

**Non-Goals:**

1. **不**修改令牌分组的核心权限模型（`middleware/auth.go` 校验逻辑不变）。
2. **不**修改用户分组（`User.Group`）的核心权限模型（`service.GetUserUsableGroups` 不变）。
3. **不**修改 classic 主题（`web/classic/`）的令牌编辑表单。如需同步，作为独立变更处理。
4. **不**为批量创建页的分组字段增加"创建新分组"的能力——分组来源仍是 `group ratio` 配置。
5. **不**改后端 `BatchCreateUsers` 接口的请求/响应结构。

## Decisions

### Decision 1: API 密钥表单 `group` 字段升级为必填，去掉预填默认值

**选择**：
- `getApiKeyFormSchema` 中 `group` 改为 `z.string().min(1, t('Please select a group'))`。
- `API_KEY_FORM_DEFAULT_VALUES.group` 从 `DEFAULT_GROUP` 改为 `''`。
- `getApiKeyFormDefaultValues(defaultUseAutoGroup)` 不再根据 `defaultUseAutoGroup` 预填 `'auto'`；初始 `group` 始终为空。
- `transformApiKeyToFormDefaults` 中 `group` 为空/null 时回填为空字符串（**不**自动映射到 `default`），让用户在编辑保存前主动选择。

**备选方案**：
- 保留 `default`/`auto` 预填，但在 `onSubmit` 前再做一次校验。→ 被否，因为预填会让用户跳过选择，与"必须显式选择"目标矛盾。
- 把空值映射到 `default` 后允许保存。→ 被否，因为新校验的本意就是消除隐式默认。

**为什么**：用户原话"用户必须选择一个分组，否则进行提示且无法下一步创建"明确了行为预期，预填会让校验形同虚设。

### Decision 2: 复用 `ApiKeyGroupCombobox` 组件，不新建管理员专用下拉

**选择**：批量创建用户表单的 `group` 字段直接使用 `ApiKeyGroupCombobox`（必要时把组件从 `features/keys/components/` 提取到 `components/` 共享位置）。

**备选方案**：
- 新建 `AdminGroupCombobox`。→ 被否，因为视觉与交互应该一致，多一份维护成本。
- 用 `@/components/ui/select` 的原生 `Select`。→ 被否，因为分组数量可能较多，需要搜索能力，`ApiKeyGroupCombobox` 已经做了搜索/倍率 badge。

**组件提取策略**：
- 当前位置：`web/default/src/features/keys/components/api-key-group-combobox.tsx`
- 提取到：`web/default/src/components/group-combobox.tsx`（重命名为 `GroupCombobox`，类型 `GroupOption`）
- 原 `ApiKeyGroupCombobox` 与 `ApiKeyGroupOption` 作为 re-export 保留，避免破坏现有 import。

### Decision 3: 新增 `GET /api/group/details` 端点供管理员批量创建页使用

**选择**：在 `controller/group.go` 新增 `GetGroupDetails`，返回 `[]{name, ratio, desc}` 结构（包含 `auto` 虚拟分组）。路由 `GET /api/group/details` 挂在现有 `groupRoute`（已有 `AdminAuth` 中间件）下。

**备选方案**：
- 扩展 `GetGroups` 返回完整结构。→ 被否，因为现有 `data: []string` 是 breaking change，破坏"前端只读名字数组"的隐式契约。
- 让管理员前端调用 `/api/user/self/groups`。→ 被否，因为该接口受 `service.GetUserUsableGroups(adminUserGroup)` 过滤，无法保证列出全部系统分组。
- 前端只用 `[]string`，下拉不带倍率。→ 被否，UX 削弱，与单创 API 密钥页不一致。

**响应结构**：
```json
{
  "success": true,
  "data": [
    {"name": "default", "ratio": 1, "desc": "默认分组"},
    {"name": "vip", "ratio": 0.5, "desc": "VIP 分组"},
    {"name": "auto", "ratio": "auto", "desc": "自动分组"}
  ]
}
```

### Decision 4: 批量创建用户表单的 `group` 字段也升级为必填

**选择**：`batchFormSchema.group` 从 `z.string().optional()` 升级为 `z.string().min(1, t('Please select a group'))`，`DEFAULT_VALUES.group` 从 `''` 保留为空（让校验生效）。

**为什么**：批量创建用户时如果不显式选分组，新用户会落到服务端 `'default'`（`model/user.go` 默认值），但管理员可能并不知道。强制选择避免"沉默默认"。

**备选**：保留 optional + placeholder。→ 被否，与 Decision 1 的精神不一致。

### Decision 5: i18n 新增键的命名

**选择**：新增以下键（英文为 key）：
- `Please select a group`（表单校验提示）
- 复用现有 `Select a group`（占位符）与 `No group found.`（搜索无结果）

`zh.json` 必须完整翻译，`fr/ja/ru/vi` 走 fallback（缺键时显示英文）。

### Decision 6 (方案 A 扩展): 批量创建页新增 token_group 字段，与 create_token 开关联动

**背景**：原有 `BatchCreateUsers` 在 `create_token=true` 时，token.Group 由 `setting.DefaultUseAutoGroup` 隐式决定（auto 或空）。与"必须显式选择"的目标不一致，管理员无法控制生成的 token 走哪个分组。

**选择**：
- 后端 `BatchCreateUsersRequest` 与 model 的 `BatchCreateUserRequest` 新增 `TokenGroup string` 字段。
- 创建 token 时优先使用 `req.TokenGroup`；若调用方未传该字段（向后兼容），则保留原 `default_use_auto_group` 决定的行为。
- Controller 层在 `create_token=true` 时校验 `token_group` 非空，返回 400。
- 前端 `batch-create-drawer.tsx` schema 新增 `token_group: z.string().optional()`，并用 `superRefine` 在 `create_token=true` 时强制必填；UI 上仅当开关 ON 时显示下拉。
- token 分组的候选项**包含** `auto`（与用户分组不同，token 分组允许 auto + 跨分组重试）。

**备选方案**：
- token.Group 直接等于用户分组（`req.Group`）。→ 被否，因为用户分组与 token 分组语义不同（管理员可能希望给一批 vip 用户配 default 分组的 token，让计费走倍率更低的渠道）。
- 保留隐式 `default_use_auto_group` 决定的行为，不新增字段。→ 被否，用户明确反馈希望可配置。

**为什么**：与单创 API 密钥页"必须显式选择"的精神一致，同时保留后端向后兼容路径（旧调用方不传 token_group 仍能工作）。

## Risks / Trade-offs

- **[风险] 现有令牌 `group` 字段为空字符串**：编辑保存时会被新校验拦截。
  → **缓解**：`transformApiKeyToFormDefaults` 把空字符串/null 保留为空，但 `onSubmit` 中如果检测到 group 为空，弹出 toast 提示"该令牌的分组未设置，请先选择一个分组再保存"。这是一次性升级成本，旧数据可在编辑时补全。
- **[风险] 现有用户 `group` 字段为空**：批量创建不影响（只针对新用户），但管理员编辑现有用户时如果 schema 联动收紧会被拦截。本次变更**不**修改管理员编辑用户的表单，避免扩大爆炸半径。
- **[风险] `ApiKeyGroupCombobox` 提取到共享位置可能引入 import 路径回归**。
  → **缓解**：在 `features/keys/components/api-key-group-combobox.tsx` 保留 re-export shims，所有现有 import 路径不变。
- **[折中] 新增 `GET /api/group/details` 端点而不是扩展 `GetGroups`**：增加了一个端点，但避免了 breaking change。可接受。
- **[折中] classic 主题不在本次范围**：classic 主题的 `EditTokenModal.jsx` 仍然允许空分组。如果项目同时维护两套主题，需要在后续独立 change 中同步。

## Migration Plan

无需数据库迁移。部署步骤：

1. 后端：新增 `GetGroupDetails` controller + 路由（增量，无 breaking）。
2. 前端：
   - 提取 `GroupCombobox` 共享组件。
   - 修改 `api-key-form.ts`（schema + 默认值）。
   - 修改 `api-keys-mutate-drawer.tsx`（去掉预填 useEffect）。
   - 修改 `batch-create-drawer.tsx`（Input → GroupCombobox + 校验）。
   - 补充 i18n 翻译。
3. 回归测试：手动验证（1）单创 API 密钥未选分组时不可提交；（2）批量创建用户页下拉正常显示倍率；（3）已有令牌编辑保存流程。

回滚策略：纯前端改动可直接 revert；后端新端点删除即可。

## Open Questions

1. **管理员编辑现有用户的分组字段**是否也需要在本次升级为下拉？默认建议**否**（保持现状，避免扩大范围），待用户确认。
2. **classic 主题**是否需要同步？默认建议**否**，作为独立变更。
3. **批量创建用户的 `group` 是否允许 `auto`**？`auto` 是令牌分组的虚拟值，用户分组语义上不应为 `auto`。建议在 `GetGroupDetails` 中区分"用户分组候选"与"令牌分组候选"，或在批量创建页过滤掉 `auto`。
