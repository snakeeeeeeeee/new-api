package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestInputTokenDetailsResolveCacheCreationTokens(t *testing.T) {
	zero := 0
	positive := 400
	negative := -5

	tests := []struct {
		name             string
		details          InputTokenDetails
		wantTokens       int
		wantOfficialSeen bool
	}{
		{
			name:             "legacy fallback",
			details:          InputTokenDetails{CachedCreationTokens: 125},
			wantTokens:       125,
			wantOfficialSeen: false,
		},
		{
			name:             "official positive takes precedence",
			details:          InputTokenDetails{CachedCreationTokens: 125, CacheWriteTokens: &positive},
			wantTokens:       400,
			wantOfficialSeen: true,
		},
		{
			name:             "official zero takes precedence",
			details:          InputTokenDetails{CachedCreationTokens: 125, CacheWriteTokens: &zero},
			wantTokens:       0,
			wantOfficialSeen: true,
		},
		{
			name:             "official negative normalizes to zero",
			details:          InputTokenDetails{CachedCreationTokens: 125, CacheWriteTokens: &negative},
			wantTokens:       0,
			wantOfficialSeen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, officialSeen := tt.details.ResolveCacheCreationTokens()
			require.Equal(t, tt.wantTokens, tokens)
			require.Equal(t, tt.wantOfficialSeen, officialSeen)
		})
	}
}

func TestInputTokenDetailsCacheWriteTokensJSONPresence(t *testing.T) {
	var missing InputTokenDetails
	require.NoError(t, common.Unmarshal([]byte(`{"cached_creation_tokens":125}`), &missing))
	require.Nil(t, missing.CacheWriteTokens)

	var explicitZero InputTokenDetails
	require.NoError(t, common.Unmarshal([]byte(`{"cached_creation_tokens":125,"cache_write_tokens":0}`), &explicitZero))
	require.NotNil(t, explicitZero.CacheWriteTokens)
	require.Zero(t, *explicitZero.CacheWriteTokens)

	encoded, err := common.Marshal(explicitZero)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"cache_write_tokens":0`)
}
