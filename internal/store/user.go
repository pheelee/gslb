package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pheelee/gslb/internal/types"
)

// userStore implements UserStore, RoleStore, and ConfigRoleStore.
type userStore struct {
	db *sql.DB
}

// UserStore operations -----------------------------------------------------------

// CreateUser inserts a new user, generating a UUID if ID is empty.
func (s *userStore) CreateUser(ctx context.Context, user *types.User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, subject, email, name, created_at, last_login_at, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Subject, user.Email, user.Name,
		user.CreatedAt, user.LastLoginAt, user.IsActive,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user and their roles by ID.
func (s *userStore) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	user := &types.User{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, subject, email, name, created_at, last_login_at, is_active
		 FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Subject, &user.Email, &user.Name,
		&user.CreatedAt, &user.LastLoginAt, &user.IsActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query user by id: %w", err)
	}

	roles, err := s.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles
	return user, nil
}

// GetUserBySubject retrieves a user by their OIDC subject claim.
func (s *userStore) GetUserBySubject(ctx context.Context, subject string) (*types.User, error) {
	user := &types.User{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, subject, email, name, created_at, last_login_at, is_active
		 FROM users WHERE subject = ?`, subject,
	).Scan(&user.ID, &user.Subject, &user.Email, &user.Name,
		&user.CreatedAt, &user.LastLoginAt, &user.IsActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query user by subject: %w", err)
	}

	roles, err := s.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles
	return user, nil
}

// UpdateUser updates email, name, and last_login_at for an existing user.
func (s *userStore) UpdateUser(ctx context.Context, user *types.User) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE users SET email = ?, name = ?, last_login_at = ? WHERE id = ?`,
		user.Email, user.Name, user.LastLoginAt, user.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
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

// ListUsers returns all users with their roles.
func (s *userStore) ListUsers(ctx context.Context) ([]types.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, subject, email, name, created_at, last_login_at, is_active
		 FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []types.User
	for rows.Next() {
		var u types.User
		if err := rows.Scan(&u.ID, &u.Subject, &u.Email, &u.Name,
			&u.CreatedAt, &u.LastLoginAt, &u.IsActive); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Roles, err = s.GetUserRoles(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// AssignRole assigns a role to a user (idempotent).
func (s *userStore) AssignRole(ctx context.Context, userID, roleID, assignedBy string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO user_roles (user_id, role_id, assigned_at, assigned_by)
		 VALUES (?, ?, ?, ?)`,
		userID, roleID, time.Now(), assignedBy,
	)
	if err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	return nil
}

// RemoveRole removes a role from a user.
func (s *userStore) RemoveRole(ctx context.Context, userID, roleID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM user_roles WHERE user_id = ? AND role_id = ?`,
		userID, roleID,
	)
	if err != nil {
		return fmt.Errorf("remove role: %w", err)
	}
	return nil
}

// GetUserRoles retrieves all roles for a user.
func (s *userStore) GetUserRoles(ctx context.Context, userID string) ([]types.Role, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.id, r.name, r.description, r.created_at
		 FROM roles r
		 JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = ?
		 ORDER BY r.name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query user roles: %w", err)
	}
	defer rows.Close()

	var roles []types.Role
	for rows.Next() {
		var r types.Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// RoleStore operations -----------------------------------------------------------

// GetRoleByName retrieves a role by its name.
func (s *userStore) GetRoleByName(ctx context.Context, name string) (*types.Role, error) {
	r := &types.Role{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, created_at FROM roles WHERE name = ?`, name,
	).Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query role by name: %w", err)
	}
	return r, nil
}

// GetRoleByID retrieves a role by its ID.
func (s *userStore) GetRoleByID(ctx context.Context, id string) (*types.Role, error) {
	r := &types.Role{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, created_at FROM roles WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query role by id: %w", err)
	}
	return r, nil
}

// ListRoles returns all roles.
func (s *userStore) ListRoles(ctx context.Context) ([]types.Role, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, created_at FROM roles ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	var roles []types.Role
	for rows.Next() {
		var r types.Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// ConfigRoleStore operations -----------------------------------------------------

// AssignConfigRole assigns a role to a config (idempotent).
func (s *userStore) AssignConfigRole(ctx context.Context, configID, roleID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO config_roles (config_id, role_id) VALUES (?, ?)`,
		configID, roleID,
	)
	if err != nil {
		return fmt.Errorf("assign config role: %w", err)
	}
	return nil
}

// RemoveConfigRole removes a role from a config.
func (s *userStore) RemoveConfigRole(ctx context.Context, configID, roleID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM config_roles WHERE config_id = ? AND role_id = ?`,
		configID, roleID,
	)
	if err != nil {
		return fmt.Errorf("remove config role: %w", err)
	}
	return nil
}

// GetConfigRoles retrieves the roles that have access to a config.
func (s *userStore) GetConfigRoles(ctx context.Context, configID string) ([]types.Role, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.id, r.name, r.description, r.created_at
		 FROM roles r
		 JOIN config_roles cr ON cr.role_id = r.id
		 WHERE cr.config_id = ?
		 ORDER BY r.name`,
		configID,
	)
	if err != nil {
		return nil, fmt.Errorf("query config roles: %w", err)
	}
	defer rows.Close()

	var roles []types.Role
	for rows.Next() {
		var r types.Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan config role: %w", err)
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// GetConfigsForRole returns config IDs accessible to a role.
func (s *userStore) GetConfigsForRole(ctx context.Context, roleID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT config_id FROM config_roles WHERE role_id = ?`, roleID,
	)
	if err != nil {
		return nil, fmt.Errorf("query configs for role: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan config id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CanAccess checks whether a user has access to a config via their roles.
func (s *userStore) CanAccess(ctx context.Context, userID, configID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM config_roles cr
		 JOIN user_roles ur ON ur.role_id = cr.role_id
		 WHERE ur.user_id = ? AND cr.config_id = ?`,
		userID, configID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check access: %w", err)
	}
	return count > 0, nil
}

// ListConfigsForUser returns configs accessible to a user via their roles.
func (s *userStore) ListConfigsForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT cr.config_id
		 FROM config_roles cr
		 JOIN user_roles ur ON ur.role_id = cr.role_id
		 WHERE ur.user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query configs for user: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan config id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
