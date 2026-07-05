package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateGeneratedUsername 校验新生成的用户名格式：
// {prefix}{date_suffix}{6位字母数字随机字符}，总长 ≤ 20
func validateGeneratedUsername(t *testing.T, username, prefix, dateSuffix string) {
	t.Helper()
	expectedPrefix := prefix + dateSuffix
	assert.Truef(t, strings.HasPrefix(username, expectedPrefix),
		"用户名 %q 应以 %q 开头", username, expectedPrefix)
	suffix := strings.TrimPrefix(username, expectedPrefix)
	assert.Lenf(t, suffix, 6, "用户名 %q 的随机后缀长度应为 6，实际 %d", username, len(suffix))
	for _, r := range suffix {
		assert.Truef(t, (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'),
			"用户名 %q 的随机后缀应为字母数字字符，包含 %q", username, r)
	}
	assert.LessOrEqualf(t, len(username), 20, "用户名 %q 长度应 ≤ 20", username)
}

func TestBatchCreateUsers_Success(t *testing.T) {
	users, err := BatchCreateUsers(BatchCreateUserRequest{
		Prefix:      "test",
		DateSuffix:  "0601",
		Count:       3,
		Group:       "default",
		Role:        common.RoleCommonUser,
		CreateToken: true,
	})
	require.NoError(t, err)
	assert.Len(t, users, 3)

	// 每个用户名都符合 {prefix}{date_suffix}{6位随机} 格式
	for _, u := range users {
		validateGeneratedUsername(t, u.Username, "test", "0601")
	}

	// 批量内用户名应两两不同
	seen := make(map[string]bool, len(users))
	for _, u := range users {
		require.Falsef(t, seen[u.Username], "批量内用户名重复: %s", u.Username)
		seen[u.Username] = true
	}
}

func TestBatchCreateUsers_RandomSuffixAvoidsConflict(t *testing.T) {
	// 第一次批量：成功创建 5 个用户
	users1, err := BatchCreateUsers(BatchCreateUserRequest{
		Prefix:     "rdc",
		DateSuffix: "0601",
		Count:      5,
	})
	require.NoError(t, err)
	assert.Len(t, users1, 5)

	// 第二次批量：相同 prefix+date_suffix，应自动生成不冲突的新用户名
	users2, err := BatchCreateUsers(BatchCreateUserRequest{
		Prefix:     "rdc",
		DateSuffix: "0601",
		Count:      5,
	})
	require.NoError(t, err)
	assert.Len(t, users2, 5)

	// 第二批的所有用户名都不应与第一批冲突
	firstBatch := make(map[string]bool, len(users1))
	for _, u := range users1 {
		firstBatch[u.Username] = true
	}
	for _, u := range users2 {
		assert.Falsef(t, firstBatch[u.Username], "用户名 %s 在两次批量中重复", u.Username)
	}
}

func TestBatchCreateUsers_TooLongPrefix(t *testing.T) {
	// 前缀 12 字符 + 4 字符日期 + 6 随机 = 22 > 20，应失败
	_, err := BatchCreateUsers(BatchCreateUserRequest{
		Prefix:     "verylongprefix",
		DateSuffix: "0601",
		Count:      1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户名过长")
}

func TestBatchCreateUsers_PrefixOnly(t *testing.T) {
	// 不传 date_suffix：用户名 = prefix + 6位随机
	users, err := BatchCreateUsers(BatchCreateUserRequest{
		Prefix:     "plain",
		DateSuffix: "",
		Count:      2,
	})
	require.NoError(t, err)
	assert.Len(t, users, 2)
	for _, u := range users {
		validateGeneratedUsername(t, u.Username, "plain", "")
	}
}

// createTestUserForBatch 创建并返回一个独立测试用户（不设置固定 ID，依赖自增）。
// 用户名与 aff_code 都带唯一值，避免与其他测试冲突（aff_code 有 UNIQUE 约束）。
func createTestUserForBatch(t *testing.T, username string) *User {
	t.Helper()
	u := &User{
		Username: username,
		Password: "testpwd123",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "aff_" + username, // 必须唯一：aff_code 有 UNIQUE 约束
	}
	require.NoError(t, DB.Create(u).Error)
	return u
}

// TestGetBatchUserSubscriptionStatuses 验证批量订阅状态查询的去重、active 过期复核、状态排序。
// 覆盖以下场景：
//   - 用户有 active+pending 多条订阅 → 输出 "active,pending"
//   - active 订阅 end_time 已过 → 复核为 "expired"
//   - 仅 cancelled 订阅 → SQL 已过滤，map 中不存在该用户
//   - 无任何订阅 → map 中不存在
func TestGetBatchUserSubscriptionStatuses(t *testing.T) {
	now := common.GetTimestamp()

	// 用户 A：active（未过期）+ pending
	userA := createTestUserForBatch(t, "batch_subs_a")
	require.NoError(t, DB.Create(&UserSubscription{
		UserId: userA.Id, Status: "active", StartTime: now - 100, EndTime: now + 3600,
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId: userA.Id, Status: "pending", StartTime: 0, EndTime: 0,
	}).Error)

	// 用户 B：active 但 end_time 已过 → 应转为 expired
	userB := createTestUserForBatch(t, "batch_subs_b")
	require.NoError(t, DB.Create(&UserSubscription{
		UserId: userB.Id, Status: "active", StartTime: now - 7200, EndTime: now - 3600,
	}).Error)

	// 用户 C：仅 cancelled → SQL 中过滤掉，map 不应包含
	userC := createTestUserForBatch(t, "batch_subs_c")
	require.NoError(t, DB.Create(&UserSubscription{
		UserId: userC.Id, Status: "cancelled", StartTime: now - 7200, EndTime: now - 3600,
	}).Error)

	// 用户 D：无任何订阅
	userD := createTestUserForBatch(t, "batch_subs_d")

	result, err := GetBatchUserSubscriptionStatuses([]int{userA.Id, userB.Id, userC.Id, userD.Id})
	require.NoError(t, err)

	// A: active + pending → "active,pending"
	assert.Equal(t, "active,pending", result[userA.Id], "用户 A 应为 active,pending")
	// B: active 过期 → "expired"
	assert.Equal(t, "expired", result[userB.Id], "用户 B active 已过期应转为 expired")
	// C: 仅 cancelled，SQL 已过滤 → map 中不存在
	_, existsC := result[userC.Id]
	assert.False(t, existsC, "用户 C 仅 cancelled 应被 SQL 过滤掉")
	// D: 无订阅 → map 中不存在
	_, existsD := result[userD.Id]
	assert.False(t, existsD, "用户 D 无订阅不应出现在 map 中")
}

// TestGetBatchFirstUserTokenKeys 验证每个用户取最早创建（最小 id）的 token。
// 覆盖以下场景：
//   - 同一用户有多个 token → 返回最先创建的（id 最小）
//   - 用户只有一个 token → 返回该 token
//   - 用户无 token → map 中不存在
//   - 空 userIds 切片 → 返回空 map，无错误
func TestGetBatchFirstUserTokenKeys(t *testing.T) {
	// 用户 X：先创建 token1，再创建 token2 → 应返回 token1
	userX := createTestUserForBatch(t, "batch_tokens_x")
	token1 := &Token{
		UserId:      userX.Id,
		Key:         "sk-batch_first_x_1",
		Name:        "older",
		Status:      1,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(token1).Error)
	token2 := &Token{
		UserId:      userX.Id,
		Key:         "sk-batch_first_x_2",
		Name:        "newer",
		Status:      1,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(token2).Error)
	require.Less(t, token1.Id, token2.Id, "token1 应先创建，id 更小")

	// 用户 Y：仅一个 token
	userY := createTestUserForBatch(t, "batch_tokens_y")
	tokenY := &Token{
		UserId:      userY.Id,
		Key:         "sk-batch_first_y_only",
		Name:        "y",
		Status:      1,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(tokenY).Error)

	// 用户 Z：无 token
	userZ := createTestUserForBatch(t, "batch_tokens_z")

	result, err := GetBatchFirstUserTokenKeys([]int{userX.Id, userY.Id, userZ.Id})
	require.NoError(t, err)
	assert.Equal(t, "sk-batch_first_x_1", result[userX.Id], "用户 X 应取最早创建的 token1")
	assert.Equal(t, "sk-batch_first_y_only", result[userY.Id], "用户 Y 唯一 token")
	_, existsZ := result[userZ.Id]
	assert.False(t, existsZ, "用户 Z 无 token 不应在 map 中")

	// 空 userIds 切片
	emptyResult, err := GetBatchFirstUserTokenKeys(nil)
	require.NoError(t, err)
	assert.Empty(t, emptyResult)
}
