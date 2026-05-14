package dns

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cloudflare/cloudflare-go"
)

// CloudflareConfig represents the configuration for Cloudflare provider
type CloudflareConfig struct {
	APIToken string `json:"api_token"`
	ZoneID   string `json:"zone_id"`
}

// CloudflareProvider implements DNSProvider for Cloudflare
type CloudflareProvider struct {
	client *cloudflare.API
	zoneID string
}

// NewCloudflareProvider creates a new Cloudflare provider from JSON config
func NewCloudflareProvider(configJSON string) (DNSProvider, error) {
	var cfg CloudflareConfig
	if err := parseConfig(configJSON, &cfg); err != nil {
		return nil, err
	}

	if cfg.APIToken == "" {
		return nil, fmt.Errorf("api_token is required for Cloudflare provider")
	}
	if cfg.ZoneID == "" {
		return nil, fmt.Errorf("zone_id is required for Cloudflare provider")
	}

	client, err := cloudflare.NewWithAPIToken(cfg.APIToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare client: %w", err)
	}

	return &CloudflareProvider{
		client: client,
		zoneID: cfg.ZoneID,
	}, nil
}

// Name returns the provider identifier
func (p *CloudflareProvider) Name() string {
	return "cloudflare"
}

// UpsertRecord creates or updates DNS A records using a set-diff approach:
// records not in the desired set are deleted, missing records are created,
// and existing records with a mismatched TTL are updated in place.
func (p *CloudflareProvider) UpsertRecord(ctx context.Context, req UpsertRequest) error {
	if err := ValidateUpsertRequest(req); err != nil {
		return err
	}

	// Use provider's zone ID if not specified in request
	zoneID := req.ZoneID
	if zoneID == "" {
		zoneID = p.zoneID
	}

	rc := cloudflare.ResourceIdentifier(zoneID)

	existing, _, err := p.client.ListDNSRecords(ctx, rc, cloudflare.ListDNSRecordsParams{
		Name: req.Name,
		Type: "A",
	})
	if err != nil {
		return fmt.Errorf("failed to list DNS records: %w", err)
	}

	// Index existing records by their IP content for O(1) lookup.
	existingByIP := make(map[string]cloudflare.DNSRecord, len(existing))
	for _, r := range existing {
		existingByIP[r.Content] = r
	}

	// Build desired IP set.
	desired := make(map[string]struct{}, len(req.Records))
	for _, ip := range req.Records {
		desired[ip] = struct{}{}
	}

	// Delete records that are no longer wanted.
	for ip, rec := range existingByIP {
		if _, keep := desired[ip]; !keep {
			if err := p.client.DeleteDNSRecord(ctx, rc, rec.ID); err != nil {
				return fmt.Errorf("failed to delete DNS record %s: %w", ip, err)
			}
		}
	}

	// Create or update records to match desired state.
	for _, ip := range req.Records {
		if rec, exists := existingByIP[ip]; exists {
			// Record exists — only update if TTL differs.
			if rec.TTL != req.TTL {
				if _, err := p.client.UpdateDNSRecord(ctx, rc, cloudflare.UpdateDNSRecordParams{
					ID:      rec.ID,
					Name:    req.Name,
					Type:    "A",
					Content: ip,
					TTL:     req.TTL,
				}); err != nil {
					return fmt.Errorf("failed to update DNS record %s: %w", ip, err)
				}
			}
		} else {
			// Record does not exist — create it.
			if _, err := p.client.CreateDNSRecord(ctx, rc, cloudflare.CreateDNSRecordParams{
				Name:    req.Name,
				Type:    "A",
				Content: ip,
				TTL:     req.TTL,
			}); err != nil {
				return fmt.Errorf("failed to create DNS record %s: %w", ip, err)
			}
		}
	}

	return nil
}

// DeleteRecord removes a DNS record
func (p *CloudflareProvider) DeleteRecord(ctx context.Context, req DeleteRequest) error {
	if err := ValidateDeleteRequest(req); err != nil {
		return err
	}

	// Use provider's zone ID if not specified in request
	zoneID := req.ZoneID
	if zoneID == "" {
		zoneID = p.zoneID
	}

	rc := cloudflare.ResourceIdentifier(zoneID)

	// Find the record
	listParams := cloudflare.ListDNSRecordsParams{
		Name: req.Name,
		Type: "A",
	}

	records, _, err := p.client.ListDNSRecords(ctx, rc, listParams)
	if err != nil {
		return fmt.Errorf("failed to list DNS records: %w", err)
	}

	if len(records) == 0 {
		// Nothing to delete — treat as success (idempotent).
		return nil
	}

	// Delete all matching records
	for _, record := range records {
		err := p.client.DeleteDNSRecord(ctx, rc, record.ID)
		if err != nil {
			// Check if it's a "not found" error
			if isNotFoundError(err) {
				continue
			}
			return fmt.Errorf("failed to delete DNS record: %w", err)
		}
	}

	return nil
}

// isNotFoundError checks if an error is a "not found" error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for HTTP 404
	if cfErr, ok := err.(*cloudflare.Error); ok {
		return cfErr.StatusCode == http.StatusNotFound
	}
	return false
}
