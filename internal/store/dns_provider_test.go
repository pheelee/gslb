package store

import (
	"context"
	"testing"

	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newConfigForProvider(t *testing.T, s Store) *types.Config {
	t.Helper()
	cfg := &types.Config{
		Name:     "dns-test-config",
		DNSName:  "dns-test.example.com",
		DNSTTL:   300,
		LBMethod: types.RoundRobin,
	}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))
	return cfg
}

func TestDNSProviderCRUD(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()
	cfg := newConfigForProvider(t, s)

	provider := &types.DNSProviderConfig{
		ConfigID:     cfg.ID,
		ProviderType: "mock",
		ConfigJSON:   `{"zone_id":"test-zone"}`,
	}

	t.Run("CreateDNSProvider", func(t *testing.T) {
		err := s.CreateDNSProvider(ctx, provider)
		require.NoError(t, err)
		assert.NotEmpty(t, provider.ID)
	})

	t.Run("GetDNSProvider", func(t *testing.T) {
		got, err := s.GetDNSProvider(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, provider.ID, got.ID)
		assert.Equal(t, "mock", got.ProviderType)
		assert.Equal(t, `{"zone_id":"test-zone"}`, got.ConfigJSON)
	})

	t.Run("GetDNSProvider_NotFound", func(t *testing.T) {
		_, err := s.GetDNSProvider(ctx, "nonexistent-cfg")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("UpdateDNSProvider", func(t *testing.T) {
		provider.ProviderType = "cloudflare"
		provider.ConfigJSON = `{"api_token":"xyz","zone_id":"z1"}`
		err := s.UpdateDNSProvider(ctx, provider)
		require.NoError(t, err)

		got, err := s.GetDNSProvider(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, "cloudflare", got.ProviderType)
		assert.Equal(t, `{"api_token":"xyz","zone_id":"z1"}`, got.ConfigJSON)
	})

	t.Run("UpdateDNSProvider_NotFound", func(t *testing.T) {
		err := s.UpdateDNSProvider(ctx, &types.DNSProviderConfig{
			ConfigID:     "nonexistent",
			ProviderType: "mock",
			ConfigJSON:   "{}",
		})
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("DeleteDNSProvider", func(t *testing.T) {
		err := s.DeleteDNSProvider(ctx, cfg.ID)
		require.NoError(t, err)

		_, err = s.GetDNSProvider(ctx, cfg.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("DeleteDNSProvider_NotFound", func(t *testing.T) {
		err := s.DeleteDNSProvider(ctx, "nonexistent-cfg")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestDNSProvider_ForeignKeyViolation(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	err := s.CreateDNSProvider(ctx, &types.DNSProviderConfig{
		ConfigID:     "nonexistent-config",
		ProviderType: "mock",
		ConfigJSON:   "{}",
	})
	assert.ErrorIs(t, err, ErrForeignKeyViolation)
}
