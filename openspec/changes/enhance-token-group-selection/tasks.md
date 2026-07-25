## 1. 后端：新增分组详情端点

- [x] 1.1 在 `controller/group.go` 新增 `GetGroupDetails` 处理函数，从 `ratio_setting.GetGroupRatioCopy()` 读取所有分组，组装成 `[]{name, ratio, desc}` 数组（按 `name` 字典序排序），通过 `common.ApiSuccess` 返回
- [x] 1.2 在 `router/api-router.go` 的 `groupRoute`（已有 `AdminAuth`）下注册 `groupRoute.GET("/details", controller.GetGroupDetails)`
- [x] 1.3 验证：用管理员 token 调用 `GET /api/group/details`，响应结构与 spec 一致；用普通用户 token 调用应被 `AdminAuth` 拦截返回 403；`GET /api/group/` 仍返回字符串数组（向后兼容）

## 2. 前端：提取共享 GroupCombobox 组件

- [x] 2.1 将 `web/default/src/features/keys/components/api-key-group-combobox.tsx` 的实现复制到新文件 `web/default/src/components/group-combobox.tsx`，导出名重命名为 `GroupCombobox` 与 `GroupOption`
- [x] 2.2 把 `features/keys/components/api-key-group-combobox.tsx` 内容替换为 re-export shim：`export { GroupCombobox as ApiKeyGroupCombobox, type GroupOption as ApiKeyGroupOption } from '@/components/group-combobox'`
- [x] 2.3 验证：`bun run build` 通过；`api-keys-mutate-drawer.tsx` 现有 import 无需修改即可工作

## 3. 前端：收紧 API 密钥表单分组校验

- [x] 3.1 修改 `web/default/src/features/keys/lib/api-key-form.ts`：
  - `getApiKeyFormSchema` 中 `group` 改为 `z.string().min(1, t('Please select a group'))`
  - `API_KEY_FORM_DEFAULT_VALUES.group` 从 `DEFAULT_GROUP` 改为 `''`
  - `getApiKeyFormDefaultValues(defaultUseAutoGroup)` 不再根据 `defaultUseAutoGroup` 预填 `'auto'`，统一返回 `group: ''`
  - `transformApiKeyToFormDefaults` 中 `group` 为空/null 时回填为 `''`（不映射到 `default`）
- [x] 3.2 修改 `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx`：删除"基于 `defaultUseAutoGroup && backendHasAuto` 的 useEffect 预填分支"（148-152 行的 form.reset 调用简化为统一 `getApiKeyFormDefaultValues(false)`）
- [x] 3.3 验证：未选分组时点击"保存"显示错误提示且不发请求；选择分组后正常提交；编辑历史空分组令牌时也被校验拦截

## 4. 前端：批量创建用户表单分组字段升级

- [x] 4.1 在 `web/default/src/lib/api.ts`（或合适的 api helper）新增 `getAdminGroupDetails` 函数，调用 `GET /api/group/details`，返回 `[]{name, ratio, desc}`
- [x] 4.2 修改 `web/default/src/features/users/components/batch-create-drawer.tsx`：
  - 新增 useQuery 拉取 `getAdminGroupDetails`（仅在 `open` 时启用）
  - 把候选项转换为 `GroupOption[]` 格式，**过滤掉 `name === 'auto'`**
  - 把 `group` 字段的 `<Input>` 替换为 `<GroupCombobox>`，绑定 `field.value` 与 `field.onChange`
  - `batchFormSchema.group` 改为 `z.string().min(1, t('Please select a group'))`
  - `DEFAULT_VALUES.group` 保留为 `''`
- [x] 4.3 验证：管理员打开抽屉看到下拉（含倍率 badge），搜索功能正常；未选分组时不可提交；`auto` 不在候选列表中；提交时 `group` 字段值与下拉选择一致

## 5. i18n：补充翻译键

- [x] 5.1 在 `web/default/src/i18n/locales/en.json` 与 `zh.json` 中新增/确认 `Please select a group` 的翻译（zh：`请选择一个分组`），以及 `auto` 过滤相关（如有）的描述键
- [x] 5.2 验证：UI 在中英文切换时校验提示与下拉占位符均正确显示

## 6. 回归测试与最终验证

- [ ] 6.1 手动回归：单创 API 密钥（未选/已选分组）、编辑历史空分组令牌、批量创建用户（未选/已选分组、搜索、提交）
- [x] 6.2 验证 classic 主题未受影响（`web/classic/` 的 `EditTokenModal.jsx` 保持原状）
- [x] 6.3 运行 `go build ./...` 与 `bun run build`（在 `web/default/`），确保后端编译通过、前端构建无错误
- [ ] 6.4 在 SQLite/MySQL 任一数据库上验证 `BatchCreateUsers` 与新端点（无数据库改动，仅需冒烟测试）

## 7. 方案 A 扩展：批量创建页新增 token_group 字段

- [x] 7.1 后端 `controller/user.go`：`BatchCreateUsersRequest` 新增 `TokenGroup string` 字段；在 `BatchCreateUsers` 处理函数中，当 `req.CreateToken == true` 时校验 `TokenGroup` 非空，否则返回 400；将 `TokenGroup` 透传给 model 层
- [x] 7.2 后端 `model/user.go`：`BatchCreateUserRequest` 新增 `TokenGroup string` 字段；`BatchCreateUsers` 创建 token 时优先使用 `req.TokenGroup`，若为空则保留原 `default_use_auto_group` 决定的行为（向后兼容）
- [x] 7.3 前端 `batch-create-drawer.tsx`：
  - schema 新增 `token_group: z.string().optional()`，用 `superRefine` 在 `create_token=true` 时强制必填
  - `DEFAULT_VALUES` 增加 `token_group: ''`
  - 在 `create_token` 开关下方新增"API 密钥分组"`GroupCombobox`，仅在开关 ON 时渲染
  - 候选项复用 `getAdminGroupDetails` 数据源，**包含** `auto`（与用户分组下拉不同）
- [x] 7.4 i18n：新增 `API Token Group`、`Select a token group`、`Please select a token group`、`Select the API token group assigned to each created token` 等键的中英文翻译
- [x] 7.5 验证：`go build ./...` + `bun run tsc --noEmit` 通过；浏览器在批量创建页勾选"创建 API 密钥"后看到 token 分组下拉，未选时被校验拦截
