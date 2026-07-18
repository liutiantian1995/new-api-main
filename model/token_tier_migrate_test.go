package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenTierMigrateAndCRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}))

	ch := Channel{
		MaxTokens: 200000,
		TokenTiers: []TokenTier{
			{MaxTokens: 50000, PriorityBoost: 5},
			{MaxTokens: 200000, PriorityBoost: 3},
		},
	}
	require.NoError(t, db.Create(&ch).Error)
	require.NotZero(t, ch.Id)

	var got Channel
	require.NoError(t, db.First(&got, ch.Id).Error)
	require.Equal(t, 200000, got.MaxTokens)
	require.Len(t, got.TokenTiers, 2)
	require.Equal(t, int64(5), got.TokenTiers[0].PriorityBoost)
	require.Equal(t, 200000, got.TokenTiers[1].MaxTokens)
}

func TestTokenTierDefaults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}))

	ch := Channel{Name: "no-token-config"}
	require.NoError(t, db.Create(&ch).Error)

	var got Channel
	require.NoError(t, db.First(&got, ch.Id).Error)
	require.Equal(t, 0, got.MaxTokens)
	require.Empty(t, got.TokenTiers)
}
