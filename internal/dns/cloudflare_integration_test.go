package dns

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/require"
)

// TestCloudflareIntegration tests real Cloudflare DNS updates.
// Requires CF_ZONE_ID and CF_TOKEN environment variables.
// Skipped automatically when those vars are absent.
func TestCloudflareIntegration(t *testing.T) {
	zoneID := os.Getenv("CF_ZONE_ID")
	token := os.Getenv("CF_TOKEN")
	if zoneID == "" || token == "" {
		t.Skip("CF_ZONE_ID and CF_TOKEN not set")
	}

	configJSON := fmt.Sprintf(`{"api_token":%q,"zone_id":%q}`, token, zoneID)
	p, err := NewCloudflareProvider(configJSON)
	require.NoError(t, err)

	ctx := context.Background()
	// The zone is byoi.ch — use a name within that zone.
	const testName = "gslb-test.byoi.ch"

	// Clean up any stale records from previous runs (both names in case of leftover state).
	t.Run("cleanup_stale", func(t *testing.T) {
		cp := p.(*CloudflareProvider)
		rc := cloudflare.ResourceIdentifier(cp.zoneID)
		for _, name := range []string{testName, "gslb-test.irbe.ch.byoi.ch"} {
			recs, _, err := cp.client.ListDNSRecords(ctx, rc, cloudflare.ListDNSRecordsParams{Name: name})
			require.NoError(t, err)
			for _, r := range recs {
				t.Logf("deleting stale record: name=%s id=%s content=%s", r.Name, r.ID, r.Content)
				_ = cp.client.DeleteDNSRecord(ctx, rc, r.ID)
			}
		}
	})

	t.Run("upsert_creates_record", func(t *testing.T) {
		err := p.UpsertRecord(ctx, UpsertRequest{
			Name:    testName,
			TTL:     60,
			Records: []string{"1.2.3.4"},
		})
		require.NoError(t, err, "upsert with single IP should succeed")
	})

	t.Run("upsert_updates_record", func(t *testing.T) {
		err := p.UpsertRecord(ctx, UpsertRequest{
			Name:    testName,
			TTL:     60,
			Records: []string{"1.2.3.4", "5.6.7.8"},
		})
		require.NoError(t, err, "upsert with two IPs should succeed")
	})

	t.Run("delete_removes_record", func(t *testing.T) {
		err := p.DeleteRecord(ctx, DeleteRequest{Name: testName})
		require.NoError(t, err, "delete should succeed")
	})

	t.Run("delete_nonexistent_record", func(t *testing.T) {
		// Deleting a record that doesn't exist should not panic
		err := p.DeleteRecord(ctx, DeleteRequest{Name: testName})
		// Either succeeds (no-op) or returns an error — just must not panic
		t.Logf("delete nonexistent: %v", err)
	})
}
