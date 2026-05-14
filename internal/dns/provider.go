// Package dns provides DNS provider abstractions and implementations
package dns

import (
	"context"
	"encoding/json"
	"fmt"
)

// DNSProvider abstracts DNS operations
type DNSProvider interface {
	// UpsertRecord creates or updates a DNS A record
	UpsertRecord(ctx context.Context, req UpsertRequest) error

	// DeleteRecord removes a DNS record
	DeleteRecord(ctx context.Context, req DeleteRequest) error

	// Name returns provider identifier
	Name() string
}

// UpsertRequest represents a request to create or update a DNS record
type UpsertRequest struct {
	ZoneID  string   // Provider-specific zone identifier
	Name    string   // DNS record name (e.g., "app.example.com")
	TTL     int      // Time to live in seconds
	Records []string // IP addresses
}

// DeleteRequest represents a request to delete a DNS record
type DeleteRequest struct {
	ZoneID string // Provider-specific zone identifier
	Name   string // DNS record name
}

// ProviderFactory creates providers from config JSON
type ProviderFactory func(configJSON string) (DNSProvider, error)

// Providers is the registry of available providers
var Providers = map[string]ProviderFactory{
	"cloudflare": NewCloudflareProvider,
	"mock":       NewMockProvider,
}

// CreateProvider creates a DNS provider by type with the given configuration
func CreateProvider(providerType string, configJSON string) (DNSProvider, error) {
	factory, ok := Providers[providerType]
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
	return factory(configJSON)
}

// ValidateUpsertRequest validates an upsert request
func ValidateUpsertRequest(req UpsertRequest) error {
	// ZoneID can be empty - provider will use its configured zone ID
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.TTL <= 0 {
		return fmt.Errorf("ttl must be greater than 0")
	}
	if len(req.Records) == 0 {
		return fmt.Errorf("at least one record is required")
	}
	for _, record := range req.Records {
		if record == "" {
			return fmt.Errorf("record cannot be empty")
		}
	}
	return nil
}

// ValidateDeleteRequest validates a delete request
func ValidateDeleteRequest(req DeleteRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// parseConfig is a helper to parse JSON config into a struct
func parseConfig(configJSON string, target interface{}) error {
	if configJSON == "" {
		return fmt.Errorf("config is required")
	}
	if err := json.Unmarshal([]byte(configJSON), target); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	return nil
}
