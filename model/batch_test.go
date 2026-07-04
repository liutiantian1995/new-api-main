package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.Equal(t, "test06011", users[0].Username)
	assert.Equal(t, "test06012", users[1].Username)
	assert.Equal(t, "test06013", users[2].Username)
}

func TestBatchCreateUsers_Conflict(t *testing.T) {
	_, err := BatchCreateUsers(BatchCreateUserRequest{
		Prefix:     "conflict",
		DateSuffix: "0601",
		Count:      2,
	})
	require.NoError(t, err)
	_, err = BatchCreateUsers(BatchCreateUserRequest{
		Prefix:     "conflict",
		DateSuffix: "0601",
		Count:      2,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户名已存在")
}

func TestBatchCreateUsers_RollbackOnConflict(t *testing.T) {
	// Batch 1: creates rollback01, rollback02, rollback03 successfully
	_, err := BatchCreateUsers(BatchCreateUserRequest{
		Prefix:     "rollback",
		DateSuffix: "",
		Count:      3,
	})
	require.NoError(t, err)

	// Batch 2: same prefix -> all usernames already exist, should fail before tx
	_, err = BatchCreateUsers(BatchCreateUserRequest{
		Prefix:     "rollback",
		DateSuffix: "",
		Count:      3,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户名已存在")
}
