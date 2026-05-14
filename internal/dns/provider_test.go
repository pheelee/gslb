package dns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderInterfaceCompliance tests that all providers implement the interface
func TestProviderInterfaceCompliance(t *testing.T) {
	tests := []struct {
		name      string
		factory   ProviderFactory
		config    string
		wantError bool
	}{
		{
			name:      "mock provider",
			factory:   NewMockProvider,
			config:    `{"name": "test-mock"}`,
			wantError: false,
		},
		{
			name:      "mock provider with empty config",
			factory:   NewMockProvider,
			config:    "",
			wantError: false,
		},
		{
			name:      "cloudflare provider without token",
			factory:   NewCloudflareProvider,
			config:    `{"zone_id": "Z123"}`,
			wantError: true,
		},
		{
			name:      "cloudflare provider without zone_id",
			factory:   NewCloudflareProvider,
			config:    `{"api_token": "token123"}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := tt.factory(tt.config)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)

				// Verify interface compliance
				assert.Implements(t, (*DNSProvider)(nil), provider)
				assert.NotEmpty(t, provider.Name())
			}
		})
	}
}

// TestMockProvider_UpsertRecord tests the mock provider's UpsertRecord method
func TestMockProvider_UpsertRecord(t *testing.T) {
	ctx := context.Background()
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	mock := provider.(*MockProvider)

	tests := []struct {
		name    string
		req     UpsertRequest
		wantErr bool
	}{
		{
			name: "valid upsert",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     300,
				Records: []string{"1.2.3.4"},
			},
			wantErr: false,
		},
		{
			name: "update existing",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     600,
				Records: []string{"5.6.7.8", "9.10.11.12"},
			},
			wantErr: false,
		},
		{
			name: "empty zone_id uses default",
			req: UpsertRequest{
				Name:    "test.example.com",
				TTL:     300,
				Records: []string{"1.2.3.4"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: UpsertRequest{
				ZoneID:  "zone1",
				TTL:     300,
				Records: []string{"1.2.3.4"},
			},
			wantErr: true,
		},
		{
			name: "invalid ttl",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     0,
				Records: []string{"1.2.3.4"},
			},
			wantErr: true,
		},
		{
			name: "empty records allowed",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     300,
				Records: []string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mock.UpsertRecord(ctx, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify record was stored
				// Use default zone if ZoneID is empty
				zoneID := tt.req.ZoneID
				if zoneID == "" {
					zoneID = "mock-zone"
				}
				record, ok := mock.GetRecord(zoneID, tt.req.Name)
				assert.True(t, ok)
				assert.Equal(t, tt.req.Name, record.Name)
				assert.Equal(t, zoneID, record.ZoneID)
				assert.Equal(t, tt.req.TTL, record.TTL)
				assert.Equal(t, tt.req.Records, record.Records)
			}
		})
	}
}

// TestMockProvider_DeleteRecord tests the mock provider's DeleteRecord method
func TestMockProvider_DeleteRecord(t *testing.T) {
	ctx := context.Background()
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	mock := provider.(*MockProvider)

	// Create records first
	err = mock.UpsertRecord(ctx, UpsertRequest{
		ZoneID:  "zone1",
		Name:    "test.example.com",
		TTL:     300,
		Records: []string{"1.2.3.4"},
	})
	require.NoError(t, err)

	// Create a record with default zone (empty ZoneID)
	err = mock.UpsertRecord(ctx, UpsertRequest{
		Name:    "default-zone.example.com",
		TTL:     300,
		Records: []string{"5.6.7.8"},
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		req     DeleteRequest
		wantErr bool
	}{
		{
			name: "delete existing",
			req: DeleteRequest{
				ZoneID: "zone1",
				Name:   "test.example.com",
			},
			wantErr: false,
		},
		{
			name: "delete non-existent",
			req: DeleteRequest{
				ZoneID: "zone1",
				Name:   "nonexistent.example.com",
			},
			wantErr: true,
		},
		{
			name: "empty zone_id uses default",
			req: DeleteRequest{
				Name: "default-zone.example.com",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: DeleteRequest{
				ZoneID: "zone1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mock.DeleteRecord(ctx, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify record was deleted
				// Use default zone if ZoneID is empty
				zoneID := tt.req.ZoneID
				if zoneID == "" {
					zoneID = "mock-zone"
				}
				_, ok := mock.GetRecord(zoneID, tt.req.Name)
				assert.False(t, ok)
			}
		})
	}
}

// TestMockProvider_RecordOperations tests record operations
func TestMockProvider_RecordOperations(t *testing.T) {
	ctx := context.Background()
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	mock := provider.(*MockProvider)

	// Test empty state
	assert.Equal(t, 0, mock.RecordCount())
	assert.Empty(t, mock.GetAllRecords())

	// Add multiple records
	records := []UpsertRequest{
		{
			ZoneID:  "zone1",
			Name:    "app1.example.com",
			TTL:     300,
			Records: []string{"1.2.3.4"},
		},
		{
			ZoneID:  "zone1",
			Name:    "app2.example.com",
			TTL:     600,
			Records: []string{"5.6.7.8", "9.10.11.12"},
		},
		{
			ZoneID:  "zone2",
			Name:    "app1.other.com",
			TTL:     300,
			Records: []string{"1.1.1.1"},
		},
	}

	for _, req := range records {
		err := mock.UpsertRecord(ctx, req)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, mock.RecordCount())
	assert.Len(t, mock.GetAllRecords(), 3)

	// Clear all records
	mock.Clear()
	assert.Equal(t, 0, mock.RecordCount())
	assert.Empty(t, mock.GetAllRecords())
}

// TestValidateUpsertRequest tests upsert request validation
func TestValidateUpsertRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     UpsertRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     300,
				Records: []string{"1.2.3.4"},
			},
			wantErr: false,
		},
		{
			name: "multiple records",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     300,
				Records: []string{"1.2.3.4", "5.6.7.8"},
			},
			wantErr: false,
		},
		{
			name: "missing zone_id",
			req: UpsertRequest{
				Name:    "test.example.com",
				TTL:     300,
				Records: []string{"1.2.3.4"},
			},
			// ZoneID is optional — provider uses its configured zone ID.
			wantErr: false,
		},
		{
			name: "missing name",
			req: UpsertRequest{
				ZoneID:  "zone1",
				TTL:     300,
				Records: []string{"1.2.3.4"},
			},
			wantErr: true,
		},
		{
			name: "zero ttl",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     0,
				Records: []string{"1.2.3.4"},
			},
			wantErr: true,
		},
		{
			name: "negative ttl",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     -1,
				Records: []string{"1.2.3.4"},
			},
			wantErr: true,
		},
		{
			name: "empty records",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     300,
				Records: []string{},
			},
			wantErr: true,
		},
		{
			name: "nil records",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     300,
				Records: nil,
			},
			wantErr: true,
		},
		{
			name: "empty record value",
			req: UpsertRequest{
				ZoneID:  "zone1",
				Name:    "test.example.com",
				TTL:     300,
				Records: []string{"1.2.3.4", ""},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUpsertRequest(tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDeleteRequest tests delete request validation
func TestValidateDeleteRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     DeleteRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: DeleteRequest{
				ZoneID: "zone1",
				Name:   "test.example.com",
			},
			wantErr: false,
		},
		{
			name: "missing zone_id",
			req: DeleteRequest{
				Name: "test.example.com",
			},
			// ZoneID is optional — provider uses its configured zone ID.
			wantErr: false,
		},
		{
			name: "missing name",
			req: DeleteRequest{
				ZoneID: "zone1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeleteRequest(tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestProvidersRegistry tests the Providers registry
func TestProvidersRegistry(t *testing.T) {
	// Test that all expected providers are registered
	assert.Contains(t, Providers, "cloudflare")
	assert.Contains(t, Providers, "mock")

	// Test CreateProvider with valid types
	_, err := CreateProvider("mock", `{"name": "test"}`)
	assert.NoError(t, err)

	// Test CreateProvider with unknown type
	_, err = CreateProvider("unknown", `{}`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider type")
}

// TestCreateProvider tests the CreateProvider function
func TestCreateProvider(t *testing.T) {
	tests := []struct {
		name      string
		provType  string
		config    string
		wantError bool
	}{
		{
			name:      "create mock provider",
			provType:  "mock",
			config:    `{"name": "test"}`,
			wantError: false,
		},
		{
			name:      "create unknown provider",
			provType:  "unknown",
			config:    `{}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := CreateProvider(tt.provType, tt.config)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}
