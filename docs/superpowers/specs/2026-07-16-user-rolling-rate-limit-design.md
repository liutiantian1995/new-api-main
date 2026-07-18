# User Rolling Rate Limit — Design Spec

- **Status**: Approved (2026-07-16)
- **Author**: AI-assisted design via brainstorming session
- **Implementation owner**: TBD
- **Depends on**: existing `ModelRequestRateLimit` (coexistence, no replacement)

## 1. Goal

Add a **multi-tier rolling-window request quota** layered on top of the existing single-window `ModelRequestRateLimit`. The new layer answers the operator question:

> "Each user may call the API at most N times within any rolling 5-hour / 1-day / 1-week window. If any tier is exceeded, the request is rejected."

It is **complementary** to the existing limit, not a replacement:

| Layer | Window scale | Purpose |
|-------|--------------|---------|
| `ModelRequestRateLimit` (existing) | minute-level | Burst / rate protection |
| `UserRollingRateLimit` (new) | hour / day / week | Long-term quota |

Either layer rejecting a request short-circuits the call with HTTP 429.

## 2. Non-goals

- Token-based or cost-based limits (request count only).
- Per-user-only configuration (we support per-group defaults + per-user overrides).
- Distributed-lock-level precision on concurrent overshoot. We accept near-exact semantics, same as the existing `ModelRequestRateLimit`.
- Replacement or migration of the existing `ModelRequestRateLimit`. Both layers run in series.
- Dashboard / non-relay route coverage. The new limit is mounted on the same relay routes as the existing model rate limit.

## 3. Confirmed decisions

| # | Decision point | Choice | Rationale |
|---|----------------|--------|-----------|
| 1 | Tier structure | Administrator-configurable list of `(duration_seconds, limit)` pairs | Matches the open-ended phrasing "5h / 1d / 1w 这些"; consistent with existing `ModelRequestRateLimitGroup` map style |
| 2 | Configuration hierarchy | Per-group defaults + per-user overrides | Group-level keeps ops scalable; per-user override handles exceptional users |
| 3 | Relationship with existing limit | Coexistence (additive) | Minute-level burst protection stays on the old layer; the new layer only handles long-window quota. Zero regression risk |
| 4 | Counting strategy | Successful requests only | Matches "quota" semantics; failed-request abuse is already covered by the existing `totalCount` path |
| 5 | Scope | Model API relay routes only | Consistent with existing `ModelRequestRateLimit` mount point |
| 6 | Per-user override storage | New `users.table` column `rolling_rate_limit VARCHAR(2048)` | Limiter middleware already reads user; one extra column is free; GORM AutoMigrate handles all three DBs |
| 7 | Algorithm | Sliding-window log via Redis List / in-memory slice (matches existing `redisRateLimitHandler` + `InMemoryRateLimiter`) | Code-style consistency, minimal new patterns, YAGNI on ZSET optimization |
| 8 | Concurrent overshoot | Accepted as near-exact, no distributed lock | Same posture as existing limit; hard locking costs more than it saves |
| 9 | Tier count cap | At most 5 tiers per group/user | Prevents pathological configs that would amplify Redis RTTs |

## 4. Architecture

### 4.1 Mount point

```
Request → Auth → ModelRequestRateLimit (existing, optional)
                → UserRollingRateLimit (new)
                → relay business logic
```

`UserRollingRateLimit` is registered in `router/relay-router.go` immediately after `ModelRequestRateLimit` on every relay route that the existing limit covers.

### 4.2 New files

| Path | Purpose |
|------|---------|
| `setting/user_rolling_rate_limit.go` | Option vars, JSON (de)serialization, validation, lookup helpers |
| `middleware/user_rolling_rate_limit.go` | Gin middleware implementing the check + record flow |
| `common/rolling-rate-limit.go` | `RollingInMemoryRateLimiter` struct with `Count` / `Record` methods and its own cleanup goroutine |
| `web/default/src/features/system-settings/request-limits/rolling-rate-limit-section.tsx` | Main settings section (enable toggle + group list) |
| `web/default/src/features/system-settings/request-limits/rolling-rate-limit-visual-editor.tsx` | Per-group tier-list visual editor |
| `web/default/src/features/system-settings/request-limits/rolling-rate-limit-dialog.tsx` | Group config dialog |
| `web/default/src/features/system-settings/request-limits/rolling-rate-limit-types.ts` | Shared TS types |
| `web/classic/src/pages/Setting/RollingRateLimit/*` | Classic-theme mirror (Semi Design style) |

### 4.3 Modified files

| Path | Change |
|------|--------|
| `model/user.go` | Add `RollingRateLimit string` field with `gorm:"type:varchar(2048)"` tag |
| `model/option.go` | Register `UserRollingRateLimitEnabled` + `UserRollingRateLimitGroup` in OptionMap and update switch |
| `router/relay-router.go` | Register `middleware.UserRollingRateLimit()` after existing model-rate-limit middleware on relay routes |
| `controller/user.go` (or wherever user CRUD lives) | Accept + persist `rolling_rate_limit` on create/update |
| `web/default/src/features/system-settings/security/section-registry.tsx` | Register the new rolling-rate-limit section |
| `web/default/src/features/system-settings/types.ts` | Add new option keys to the form-values type |
| `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json` | Add new translation keys (English source) |
| User-management edit UI (default + classic themes) | Add optional `rolling_rate_limit` field with visual editor |

## 5. Data model

### 5.1 New `users` column

```go
type User struct {
    // ... existing fields ...
    RollingRateLimit string `json:"rolling_rate_limit" gorm:"type:varchar(2048)"`
}
```

Field semantics:
- `""` — no override, fall back to group default.
- Non-empty — JSON array of tiers (see §5.3). Overrides the group default entirely (no merge).
- Invalid JSON — rejected at write time by controller validation.

GORM AutoMigrate adds the column on SQLite, MySQL, and PostgreSQL without manual SQL. The `varchar(2048)` tag is the upper bound: 5 tiers × ~50 bytes = 250 bytes typical, leaving ample headroom. The column is a plain `string` with no `default:` tag, so the AutoMigrate-restart loop warned about in `AGENTS.md` (boolean default divergence across dialects) does not apply.

### 5.2 New global options

Registered in `model/option.go` exactly like `ModelRequestRateLimitEnabled` / `ModelRequestRateLimitGroup`:

| Key | Type | Default |
|-----|------|---------|
| `UserRollingRateLimitEnabled` | bool | `false` |
| `UserRollingRateLimitGroup` | JSON string | `"{}"` |

### 5.3 Tier-list JSON format

The value of `UserRollingRateLimitGroup` is a JSON object mapping group name to a tier list:

```json
{
  "default": [
    {"duration": 18000,  "limit": 500},
    {"duration": 86400,  "limit": 2000},
    {"duration": 604800, "limit": 10000}
  ],
  "vip": [
    {"duration": 18000,  "limit": 5000},
    {"duration": 86400,  "limit": 20000}
  ]
}
```

The per-user `users.rolling_rate_limit` value is the same shape as one tier list:

```json
[{"duration": 18000, "limit": 1000}, {"duration": 86400, "limit": 5000}]
```

Field semantics:
- `duration` — window length in seconds.
- `limit` — maximum successful-request count within any rolling `duration` window.

### 5.4 Validation rules (enforced at option-set and user-update time)

- Object must parse as JSON.
- Tier count per group/user ≤ 5.
- Each `duration` ≥ 60 (seconds).
- Each `duration` ≤ 2,147,483,647 (int32 max).
- Each `limit` ≥ 1.
- Each `limit` ≤ 2,147,483,647.
- No duplicate `duration` within the same tier list.
- Group name non-empty.

## 6. Request processing flow

```
┌──────────────────────────────────────────────────────────────────┐
│ Middleware: middleware.UserRollingRateLimit()                    │
├──────────────────────────────────────────────────────────────────┤
│ 1. If !setting.UserRollingRateLimitEnabled → c.Next(); return    │
│                                                                  │
│ 2. userId := c.GetInt("id")                                     │
│    user := loaded from context (already fetched by auth)        │
│                                                                  │
│ 3. Resolve effective tier list:                                 │
│    if user.RollingRateLimit != "":                              │
│        limits = parse(user.RollingRateLimit)                    │
│    else:                                                         │
│        limits = setting.UserRollingRateLimitGroup[user.Group]   │
│                                                                  │
│ 4. If len(limits) == 0 → c.Next(); return                       │
│                                                                  │
│ 5. Pre-check (read-only) all tiers:                             │
│    for tier in limits:                                          │
│        key = "rolling_limit:{userId}:{tier.duration}"           │
│        count = redis.LLen(key)  // or len(memorySlice)          │
│        if count >= tier.limit:                                  │
│            rejectWith429(tier)  // human-readable duration      │
│            return                                                │
│                                                                  │
│ 6. c.Next()  // pass to business logic                          │
│                                                                  │
│ 7. If c.Writer.Status() < 400:  // success only                 │
│        for tier in limits:                                      │
│            LPush(key, now)                                      │
│            LTrim(key, 0, tier.limit - 1)                        │
│            Expire(key, tier.duration + 10% buffer)              │
│            // in-memory: append + trim slice                    │
└──────────────────────────────────────────────────────────────────┘
```

### 6.1 Redis key scheme

- Prefix `rolling_limit:` (distinct from existing `rateLimit:` prefix to avoid collision).
- Key format: `rolling_limit:{userId}:{durationSeconds}`
- TTL: `duration + max(60s, 10% of duration)` so the key outlives the rolling window by a safe margin and self-evicts idle users.

### 6.2 In-memory fallback

Reuse the existing `InMemoryRateLimiter` (`common/rate-limit.go`) with composite keys `rolling_limit:{userId}:{duration}`. The existing implementation already does sliding-window log semantics.

The existing `InMemoryRateLimiter.Request` both checks and records atomically. The new middleware needs **check-only** during pre-check and **record-only** after success. We resolve this by:

- Adding two new methods to `InMemoryRateLimiter`:
  - `Count(key string) int` — read-only current window length.
  - `Record(key string, max int, duration int64)` — append + trim, no check.
- Leaving `Request` intact for backward compatibility with existing callers.

Cleanup: the existing `clearExpiredItems` goroutine uses a single `expirationDuration` for all keys. With a 1-week rolling tier, that duration must be at least 1 week + buffer. Two options:

- **Option α (chosen)**: run a separate cleanup goroutine scoped to rolling-limit keys (`rolling_limit:` prefix), with its own `expirationDuration` set to the longest configured tier duration + buffer. This isolates cleanup semantics from the existing minute-level limiter.
- Option β: stretch the existing `RateLimitKeyExpirationDuration` to cover the longest tier. Rejected because it would let minute-level limiter entries linger for a week.

Decision: **Option α**. Implement a sibling struct `RollingInMemoryRateLimiter` in a new file `common/rolling-rate-limit.go`, decoupled from the minute-level `InMemoryRateLimiter`. It owns its own `store` map and `clearExpiredItems` goroutine, with cleanup cadence set to the longest configured tier duration + buffer. The two limiters do not share state, avoiding cross-contamination of cleanup semantics between minute-level and week-level windows.

### 6.3 429 response shape

- HTTP 429.
- Body: OpenAI-compatible error envelope via the existing `abortWithOpenAiMessage` helper, identical style to the existing `ModelRequestRateLimit` rejection.
- Message text: localized. Includes the offending tier in human-readable form:
  - 18000s → "5 小时" / "5 hours"
  - 86400s → "1 天" / "1 day"
  - 604800s → "1 周" / "1 week"
  - 2592000s → "30 天" / "30 days"
  - Other → "`{hours}` hours" / "`{hours}` 小时" (rounded)
- Example: `"已达到 1 天内最大请求数 2000，请稍后重试"`
- No `Retry-After` header. Sliding-window rollback time depends on the oldest in-window timestamp and is not cheap to compute; omitting the header matches the existing `ModelRequestRateLimit` behavior.

### 6.4 Human-readable duration helper

New util `common.FormatRollingDuration(sec int64) string` centralized so backend messages and frontend previews share the same formatting rules. Frontend mirrors it in `rolling-rate-limit-types.ts`.

## 7. Frontend UI

### 7.1 System settings section

Mounted in the existing request-limits area (`web/default/src/features/system-settings/security/section-registry.tsx`):

- Toggle: `UserRollingRateLimitEnabled`
- Description explains the complement relationship with the existing `ModelRequestRateLimit`.
- Group table mirrors the existing `RateLimitVisualEditor` style: each row is one group with a summary of its tier list and an edit / delete action.
- Add-group and edit-group dialogs use the tier-list editor.

### 7.2 Tier-list editor

```
┌──────────────────────────────────────────────┐
│ Group: [default    ]                         │
│                                              │
│ 滑动窗口档位（最多 5 个）:                   │
│ Duration [18000 ▾] sec  Limit [500]   [删除] │
│ Duration [86400 ▾] sec  Limit [2000]  [删除] │
│ Duration [604800 ▾] sec Limit [10000] [删除] │
│ [+ 添加档位]                                 │
│                                              │
│ 快捷预设: [5 小时] [1 天] [1 周] [30 天]    │
└──────────────────────────────────────────────┘
```

- Validation mirrors §5.4 (count ≤ 5, duration ≥ 60, no duplicates, etc.).
- JSON mode toggle mirrors existing `RateLimitSection` (Code2 / Palette icons).

### 7.3 Per-user override field

In the user-management edit dialog, add an optional `RollingRateLimit` field:

- Default empty. Placeholder: `"使用组配置 ({user.Group})"`.
- "自定义配额" expander opens the same tier-list editor.
- Save serializes to JSON string.
- Clearing the field restores group-default behavior.

### 7.4 Internationalization

- All new strings added to `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json`.
- English source keys; other languages get the English fallback until translated.
- Backend messages localized through `i18n/locales/{en,zh-CN,zh-TW}.yaml`.

### 7.5 Classic-theme parity

`web/classic/` receives a mirrored settings page and per-user field using Semi Design components, matching the existing `SettingsRequestRateLimit` structure.

## 8. Testing strategy

Per AGENTS.md backend test rules: `testify/require` for setup and fatal asserts, `testify/assert` for non-fatal checks. Prefer deterministic table tests.

### 8.1 Unit tests

- `setting/user_rolling_rate_limit.go`
  - JSON parse + serialize round-trip.
  - Validation accepts valid configs and rejects: tier count > 5, duration < 60, duplicate durations, limit < 1, overflow values, malformed JSON.
  - `GetGroupRollingRateLimit` lookup returns expected tier list and `found = false` for missing groups.
- `middleware/user_rolling_rate_limit.go` tier resolution logic (mocked user / setting):
  - Per-user override takes precedence over group default.
  - Empty per-user field falls back to group default.
  - Empty group config falls through to passthrough.
- `common.FormatRollingDuration` table test covering all branches.
- `common/rolling-rate-limit.go`
  - `Count` returns 0 for unknown keys.
  - `Record` appends and trims to `max` entries.
  - `Record` followed by `Count` reflects the new entry.
  - Cleanup goroutine evicts keys whose newest timestamp is older than the configured expiration.

### 8.2 Integration tests

- Middleware + Redis miniredis (or testcontainer) covering:
  - Single tier triggers 429 at threshold.
  - Multi-tier triggers 429 on the first violating tier, with correct message.
  - Successful request records on all tiers; failed request (status ≥ 400) records on none.
  - Per-user override changes the effective threshold.
  - Concurrent overshoot is bounded by the documented near-exact semantics.
- Same suite run against the in-memory fallback path.

### 8.3 Regression tests

- Existing `ModelRequestRateLimit` behavior unchanged when `UserRollingRateLimitEnabled = false`.
- Existing `rateLimit:` Redis keys not interfered with by `rolling_limit:` keys.

### 8.4 Frontend tests

- Tier-list editor: add / remove / validate (count cap, duration duplicate detection).
- JSON ↔ visual mode round-trip.
- Per-user override field clears to group default.

### 8.5 Coverage target

≥ 80% on new Go files; frontend coverage where existing patterns already enforce it.

## 9. Open questions to resolve during implementation

- Exact list of relay routes that should carry the new middleware (mirror `ModelRequestRateLimit` mount list — enumerate during implementation).
- Whether the classic-theme user-management edit dialog already has a natural place for the new field; if not, decide where to insert it without disrupting existing layout.

## 10. Out of scope (explicit non-goals for this change)

- Token-based or cost-based limits.
- Replacing or migrating `ModelRequestRateLimit`.
- Dashboard / non-relay route coverage.
- Distributed lock for exact concurrency.
- `Retry-After` header computation.
- Audit log entries for 429 rejections (relies on existing log pipeline).
