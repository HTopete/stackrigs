package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Session represents an authenticated user session.
type Session struct {
	ID        string
	BuilderID int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

// WebAuthnCredentialRow represents a stored WebAuthn credential.
type WebAuthnCredentialRow struct {
	ID              string
	BuilderID       int64
	CredentialID    []byte
	PublicKey       []byte
	AttestationType string
	Transport       string
	SignCount       uint32
	CreatedAt       string
}

// GitHubConnection represents a builder's linked GitHub account.
type GitHubConnection struct {
	BuilderID       int64
	GitHubID        int64
	GitHubUsername  string
	GitHubAvatarURL string
	AccessToken     string
	CreatedAt       string
}

// AuthStore handles session, WebAuthn credential, and GitHub connection persistence.
type AuthStore struct {
	db *sql.DB
}

// NewAuthStore creates a new AuthStore.
func NewAuthStore(db *sql.DB) *AuthStore {
	return &AuthStore{db: db}
}

// ---- Sessions ----

// CreateSession creates a new session for the given builder and returns the session ID.
func (s *AuthStore) CreateSession(builderID int64, maxAge time.Duration) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("generating session id: %w", err)
	}

	expiresAt := time.Now().UTC().Add(maxAge)
	_, err = s.db.Exec(
		"INSERT INTO sessions (id, builder_id, expires_at) VALUES (?, ?, ?)",
		id, builderID, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("inserting session: %w", err)
	}

	return id, nil
}

// GetSession retrieves a session by ID. Returns nil if not found or expired.
func (s *AuthStore) GetSession(id string) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRow(
		"SELECT id, builder_id, expires_at, created_at FROM sessions WHERE id = ?", id,
	).Scan(&sess.ID, &sess.BuilderID, &sess.ExpiresAt, &sess.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying session: %w", err)
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		// Expired — clean up and return nil
		_, _ = s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
		return nil, nil
	}

	return sess, nil
}

// DeleteSession removes a session by ID.
func (s *AuthStore) DeleteSession(id string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// CleanExpiredSessions removes all sessions whose expires_at is in the past.
func (s *AuthStore) CleanExpiredSessions() (int64, error) {
	result, err := s.db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("cleaning expired sessions: %w", err)
	}
	return result.RowsAffected()
}

// ---- WebAuthn Credentials ----

// SaveCredential persists a WebAuthn credential for the given builder.
func (s *AuthStore) SaveCredential(cred WebAuthnCredentialRow) error {
	_, err := s.db.Exec(
		`INSERT INTO webauthn_credentials (id, builder_id, credential_id, public_key, attestation_type, transport, sign_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cred.ID, cred.BuilderID, cred.CredentialID, cred.PublicKey,
		cred.AttestationType, cred.Transport, cred.SignCount,
	)
	if err != nil {
		return fmt.Errorf("saving webauthn credential: %w", err)
	}
	return nil
}

// GetCredentialsByBuilder returns all WebAuthn credentials for a builder.
func (s *AuthStore) GetCredentialsByBuilder(builderID int64) ([]WebAuthnCredentialRow, error) {
	rows, err := s.db.Query(
		`SELECT id, builder_id, credential_id, public_key, attestation_type, transport, sign_count, created_at
		 FROM webauthn_credentials WHERE builder_id = ?`,
		builderID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying webauthn credentials: %w", err)
	}
	defer rows.Close()

	creds := make([]WebAuthnCredentialRow, 0)
	for rows.Next() {
		var c WebAuthnCredentialRow
		if err := rows.Scan(&c.ID, &c.BuilderID, &c.CredentialID, &c.PublicKey, &c.AttestationType, &c.Transport, &c.SignCount, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning webauthn credential: %w", err)
		}
		creds = append(creds, c)
	}
	return creds, nil
}

// UpdateCredentialSignCount updates the sign count for a credential after successful authentication.
func (s *AuthStore) UpdateCredentialSignCount(credentialID string, newCount uint32) error {
	_, err := s.db.Exec(
		"UPDATE webauthn_credentials SET sign_count = ? WHERE id = ?",
		newCount, credentialID,
	)
	if err != nil {
		return fmt.Errorf("updating sign count: %w", err)
	}
	return nil
}

// ---- GitHub Connections ----

// FindOrCreateBuilderByGitHub finds an existing builder by GitHub ID or creates a new one.
// If the builder already exists, it updates the avatar. The accessToken parameter is
// intentionally not persisted — it is only used at login time and discarded afterwards.
// Returns the builder ID and whether a new builder was created.
func (s *AuthStore) FindOrCreateBuilderByGitHub(githubID int64, username, displayName, avatarURL, accessToken string) (int64, bool, error) {
	// Check if a GitHub connection exists
	var builderID int64
	err := s.db.QueryRow(
		"SELECT builder_id FROM github_connections WHERE github_id = ?", githubID,
	).Scan(&builderID)

	if err == nil {
		// Existing connection — update avatar (token intentionally not stored)
		_, err = s.db.Exec(
			"UPDATE github_connections SET access_token = '', github_avatar_url = ?, github_username = ? WHERE github_id = ?",
			avatarURL, username, githubID,
		)
		if err != nil {
			return 0, false, fmt.Errorf("updating github connection: %w", err)
		}

		// Also update builder avatar if empty
		_, _ = s.db.Exec(
			"UPDATE builders SET avatar_url = ? WHERE id = ? AND avatar_url = ''",
			avatarURL, builderID,
		)

		return builderID, false, nil
	}

	if err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("querying github connection: %w", err)
	}

	// No connection found — create a new builder
	now := time.Now().UTC()
	handle := strings.ToLower(username)

	// Ensure handle is unique by appending random suffix if needed
	var existing int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM builders WHERE handle = ?", handle).Scan(&existing)
	if existing > 0 {
		suffix := make([]byte, 3)
		_, _ = rand.Read(suffix)
		handle = fmt.Sprintf("%s-%s", handle, hex.EncodeToString(suffix))
	}

	result, err := s.db.Exec(
		`INSERT INTO builders (handle, display_name, bio, avatar_url, website, github_url, twitter_url, invitation_code, created_at, updated_at)
		 VALUES (?, ?, '', ?, '', ?, '', 'github-oauth', ?, ?)`,
		handle, displayName, avatarURL, fmt.Sprintf("https://github.com/%s", username), now, now,
	)
	if err != nil {
		return 0, false, fmt.Errorf("creating builder from github: %w", err)
	}

	builderID, err = result.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("getting builder id: %w", err)
	}

	// Create the GitHub connection (token intentionally not stored — used only at login time)
	_, err = s.db.Exec(
		`INSERT INTO github_connections (builder_id, github_id, github_username, github_avatar_url, access_token)
		 VALUES (?, ?, ?, ?, '')`,
		builderID, githubID, username, avatarURL,
	)
	if err != nil {
		return 0, false, fmt.Errorf("creating github connection: %w", err)
	}

	return builderID, true, nil
}

// GetGitHubConnection retrieves the GitHub connection for a builder.
func (s *AuthStore) GetGitHubConnection(builderID int64) (*GitHubConnection, error) {
	conn := &GitHubConnection{}
	err := s.db.QueryRow(
		"SELECT builder_id, github_id, github_username, github_avatar_url, access_token, created_at FROM github_connections WHERE builder_id = ?",
		builderID,
	).Scan(&conn.BuilderID, &conn.GitHubID, &conn.GitHubUsername, &conn.GitHubAvatarURL, &conn.AccessToken, &conn.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying github connection: %w", err)
	}
	return conn, nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
