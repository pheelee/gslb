package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/pheelee/gslb/internal/crypto"
	"github.com/pheelee/gslb/internal/types"
)

// dnsProviderStore implements DNSProviderStore interface.
type dnsProviderStore struct {
	db  *sql.DB
	enc *crypto.Encryptor // nil = no encryption (passthrough)
}

// CreateDNSProvider creates a new DNS provider configuration.
func (s *dnsProviderStore) CreateDNSProvider(ctx context.Context, provider *types.DNSProviderConfig) error {
	if provider.ID == "" {
		provider.ID = uuid.New().String()
	}

	encJSON, err := s.enc.Encrypt(provider.ConfigJSON)
	if err != nil {
		return fmt.Errorf("encrypting config_json: %w", err)
	}

	query := `
		INSERT INTO dns_providers (id, config_id, provider_type, config_json)
		VALUES (?, ?, ?, ?)
	`
	_, err = s.db.ExecContext(ctx, query,
		provider.ID, provider.ConfigID, provider.ProviderType, encJSON,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
		}
		return fmt.Errorf("insert dns provider: %w", err)
	}
	return nil
}

// GetDNSProvider retrieves a DNS provider configuration by config ID.
func (s *dnsProviderStore) GetDNSProvider(ctx context.Context, configID string) (*types.DNSProviderConfig, error) {
	query := `
		SELECT id, config_id, provider_type, config_json
		FROM dns_providers
		WHERE config_id = ?
	`
	provider := &types.DNSProviderConfig{}
	err := s.db.QueryRowContext(ctx, query, configID).Scan(
		&provider.ID, &provider.ConfigID, &provider.ProviderType, &provider.ConfigJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query dns provider: %w", err)
	}

	decrypted, err := s.enc.Decrypt(provider.ConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("decrypting config_json: %w", err)
	}
	provider.ConfigJSON = decrypted

	return provider, nil
}

// UpdateDNSProvider updates an existing DNS provider configuration.
func (s *dnsProviderStore) UpdateDNSProvider(ctx context.Context, provider *types.DNSProviderConfig) error {
	encJSON, err := s.enc.Encrypt(provider.ConfigJSON)
	if err != nil {
		return fmt.Errorf("encrypting config_json: %w", err)
	}

	query := `
		UPDATE dns_providers
		SET provider_type = ?, config_json = ?
		WHERE config_id = ?
	`
	result, err := s.db.ExecContext(ctx, query,
		provider.ProviderType, encJSON, provider.ConfigID,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
		}
		return fmt.Errorf("update dns provider: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDNSProvider deletes a DNS provider configuration by config ID.
func (s *dnsProviderStore) DeleteDNSProvider(ctx context.Context, configID string) error {
	query := `DELETE FROM dns_providers WHERE config_id = ?`
	result, err := s.db.ExecContext(ctx, query, configID)
	if err != nil {
		return fmt.Errorf("delete dns provider: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
