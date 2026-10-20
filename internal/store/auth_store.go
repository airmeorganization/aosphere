package store

import (
	"database/sql"
	"fmt"
	"time"

	"aosphere/internal/db"
	"github.com/google/uuid"
)

// CreateOrganization registers a new business organization
func (s *Store) CreateOrganization(name, slug string) (*db.Organization, error) {
	id := "org_" + uuid.New().String()[:8]
	now := time.Now()
	_, err := s.db.Exec(`INSERT INTO organizations (id, name, slug, created_at) VALUES (?, ?, ?, ?)`, id, name, slug, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}
	return &db.Organization{
		ID:        id,
		Name:      name,
		Slug:      slug,
		CreatedAt: now,
	}, nil
}

// GetOrganization retrieves an organization by ID
func (s *Store) GetOrganization(orgID string) (*db.Organization, error) {
	var org db.Organization
	err := s.db.QueryRow(`SELECT id, name, slug, created_at FROM organizations WHERE id = ?`, orgID).
		Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// CreateUser creates a new user inside an organization
func (s *Store) CreateUser(orgID, email, passwordHash, name, role string) (*db.User, error) {
	id := "usr_" + uuid.New().String()[:8]
	now := time.Now()
	_, err := s.db.Exec(`INSERT INTO users (id, org_id, email, password_hash, name, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`, id, orgID, email, passwordHash, name, role, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return &db.User{
		ID:        id,
		OrgID:     orgID,
		Email:     email,
		Name:      name,
		Role:      role,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetUserByEmail finds a user by their email address
func (s *Store) GetUserByEmail(email string) (*db.User, error) {
	var u db.User
	err := s.db.QueryRow(`SELECT id, org_id, email, password_hash, name, role, is_active, created_at, updated_at
		FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.OrgID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByID finds a user by ID
func (s *Store) GetUserByID(userID string) (*db.User, error) {
	var u db.User
	err := s.db.QueryRow(`SELECT id, org_id, email, password_hash, name, role, is_active, created_at, updated_at
		FROM users WHERE id = ?`, userID).
		Scan(&u.ID, &u.OrgID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateSession generates a session record for an authenticated user
func (s *Store) CreateSession(userID, orgID, tokenHash string, duration time.Duration) (*db.Session, error) {
	id := "sess_" + uuid.New().String()[:8]
	now := time.Now()
	expiresAt := now.Add(duration)

	_, err := s.db.Exec(`INSERT INTO sessions (id, user_id, org_id, token_hash, expires_at, created_at, last_activity_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, userID, orgID, tokenHash, expiresAt, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &db.Session{
		ID:             id,
		UserID:         userID,
		OrgID:          orgID,
		TokenHash:      tokenHash,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
		LastActivityAt: now,
	}, nil
}

// GetSessionAndUser validates a token hash, checks expiration, and returns the session & user
func (s *Store) GetSessionAndUser(tokenHash string) (*db.Session, *db.User, error) {
	var sess db.Session
	var u db.User

	query := `SELECT s.id, s.user_id, s.org_id, s.token_hash, s.expires_at, s.created_at, s.last_activity_at,
		u.id, u.org_id, u.email, u.name, u.role, u.is_active
		FROM sessions s
		INNER JOIN users u ON s.user_id = u.id
		WHERE s.token_hash = ? AND s.expires_at > CURRENT_TIMESTAMP AND u.is_active = 1`

	err := s.db.QueryRow(query, tokenHash).Scan(
		&sess.ID, &sess.UserID, &sess.OrgID, &sess.TokenHash, &sess.ExpiresAt, &sess.CreatedAt, &sess.LastActivityAt,
		&u.ID, &u.OrgID, &u.Email, &u.Name, &u.Role, &u.IsActive,
	)
	if err != nil {
		return nil, nil, err
	}

	// Update last activity
	_, _ = s.db.Exec(`UPDATE sessions SET last_activity_at = CURRENT_TIMESTAMP WHERE id = ?`, sess.ID)

	return &sess, &u, nil
}

// DeleteSession invalidates a session token
func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// CreateAPIKey generates a new scoped API key for an agent or programmatic integration
func (s *Store) CreateAPIKey(orgID, userID, name, keyHash, role, allowedTools string) (*db.APIKey, error) {
	id := "key_" + uuid.New().String()[:8]
	now := time.Now()
	if allowedTools == "" {
		allowedTools = "*"
	}
	_, err := s.db.Exec(`INSERT INTO api_keys (id, org_id, user_id, name, key_hash, role, allowed_tools, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)`, id, orgID, userID, name, keyHash, role, allowedTools, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create api key: %w", err)
	}

	return &db.APIKey{
		ID:           id,
		OrgID:        orgID,
		UserID:       userID,
		Name:         name,
		Role:         role,
		AllowedTools: allowedTools,
		IsActive:     true,
		CreatedAt:    now,
	}, nil
}

// GetAPIKeyByHash authenticates an API key
func (s *Store) GetAPIKeyByHash(keyHash string) (*db.APIKey, error) {
	var k db.APIKey
	var lastUsed sql.NullTime
	var userID sql.NullString

	err := s.db.QueryRow(`SELECT id, org_id, user_id, name, role, allowed_tools, is_active, created_at, last_used_at
		FROM api_keys WHERE key_hash = ? AND is_active = 1`, keyHash).
		Scan(&k.ID, &k.OrgID, &userID, &k.Name, &k.Role, &k.AllowedTools, &k.IsActive, &k.CreatedAt, &lastUsed)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		k.UserID = userID.String
	}
	if lastUsed.Valid {
		k.LastUsedAt = &lastUsed.Time
	}

	// Update last used
	go func(id string) {
		_, _ = s.db.Exec(`UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	}(k.ID)

	return &k, nil
}

// ListAPIKeys retrieves all API keys for an organization
func (s *Store) ListAPIKeys(orgID string) ([]db.APIKey, error) {
	rows, err := s.db.Query(`SELECT id, org_id, user_id, name, role, allowed_tools, is_active, created_at, last_used_at
		FROM api_keys WHERE org_id = ? ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.APIKey
	for rows.Next() {
		var k db.APIKey
		var userID sql.NullString
		var lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.OrgID, &userID, &k.Name, &k.Role, &k.AllowedTools, &k.IsActive, &k.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		if userID.Valid {
			k.UserID = userID.String
		}
		if lastUsed.Valid {
			k.LastUsedAt = &lastUsed.Time
		}
		list = append(list, k)
	}
	return list, nil
}
