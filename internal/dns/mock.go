package dns

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockDNSRecord represents a stored DNS record in the mock provider
type MockDNSRecord struct {
	Name      string
	ZoneID    string
	Type      string
	TTL       int
	Records   []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MockProvider implements DNSProvider for testing with in-memory storage
type MockProvider struct {
	mu      sync.RWMutex
	records map[string]MockDNSRecord // key: "zoneID/name"
	name    string
	zoneID  string // default zone ID
}

// MockConfig represents the configuration for Mock provider
type MockConfig struct {
	Name string `json:"name,omitempty"`
}

// NewMockProvider creates a new Mock provider from JSON config
func NewMockProvider(configJSON string) (DNSProvider, error) {
	var cfg MockConfig
	// Config is optional for mock provider
	if configJSON != "" {
		if err := parseConfig(configJSON, &cfg); err != nil {
			// If config is invalid, just use defaults
			cfg.Name = "mock"
		}
	}

	if cfg.Name == "" {
		cfg.Name = "mock"
	}

	return &MockProvider{
		records: make(map[string]MockDNSRecord),
		name:    cfg.Name,
		zoneID:  "mock-zone",
	}, nil
}

// Name returns the provider identifier
func (p *MockProvider) Name() string {
	return p.name
}

// recordKey generates a unique key for a record
func (p *MockProvider) recordKey(zoneID, name string) string {
	return fmt.Sprintf("%s/%s", zoneID, name)
}

// UpsertRecord creates or updates a DNS A record
func (p *MockProvider) UpsertRecord(ctx context.Context, req UpsertRequest) error {
	// Use provider's zone ID if not specified in request
	zoneID := req.ZoneID
	if zoneID == "" {
		zoneID = p.zoneID
	}

	// Validate required fields (zone ID is optional for mock)
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.TTL <= 0 {
		return fmt.Errorf("ttl must be greater than 0")
	}
	for _, record := range req.Records {
		if record == "" {
			return fmt.Errorf("record cannot be empty")
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.recordKey(zoneID, req.Name)
	now := time.Now()

	if existing, ok := p.records[key]; ok {
		// Update existing record
		existing.Records = req.Records
		existing.TTL = req.TTL
		existing.UpdatedAt = now
		p.records[key] = existing
	} else {
		// Create new record
		p.records[key] = MockDNSRecord{
			Name:      req.Name,
			ZoneID:    zoneID,
			Type:      "A",
			TTL:       req.TTL,
			Records:   req.Records,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	return nil
}

// DeleteRecord removes a DNS record
func (p *MockProvider) DeleteRecord(ctx context.Context, req DeleteRequest) error {
	// Use provider's zone ID if not specified in request
	zoneID := req.ZoneID
	if zoneID == "" {
		zoneID = p.zoneID
	}

	// Validate required fields (zone ID is optional for mock)
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.recordKey(zoneID, req.Name)

	if _, ok := p.records[key]; !ok {
		return fmt.Errorf("record not found: %s", req.Name)
	}

	delete(p.records, key)
	return nil
}

// GetRecord retrieves a record by zone ID and name (for testing)
func (p *MockProvider) GetRecord(zoneID, name string) (MockDNSRecord, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := p.recordKey(zoneID, name)
	record, ok := p.records[key]
	return record, ok
}

// GetAllRecords returns all stored records (for testing)
func (p *MockProvider) GetAllRecords() []MockDNSRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()

	records := make([]MockDNSRecord, 0, len(p.records))
	for _, record := range p.records {
		records = append(records, record)
	}
	return records
}

// Clear removes all records (for testing)
func (p *MockProvider) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.records = make(map[string]MockDNSRecord)
}

// RecordCount returns the number of stored records (for testing)
func (p *MockProvider) RecordCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.records)
}
