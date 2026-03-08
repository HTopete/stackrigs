package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/htopete/stackrigs/internal/model"
)

type BuilderStore struct {
	db *sql.DB
}

func NewBuilderStore(db *sql.DB) *BuilderStore {
	return &BuilderStore{db: db}
}

func (s *BuilderStore) GetByHandle(handle string) (*model.Builder, error) {
	b := &model.Builder{}
	err := s.db.QueryRow(
		`SELECT id, handle, display_name, bio, avatar_url, website, github_url, twitter_url, created_at, updated_at
		 FROM builders WHERE handle = ?`, handle,
	).Scan(&b.ID, &b.Handle, &b.DisplayName, &b.Bio, &b.AvatarURL, &b.Website, &b.GithubURL, &b.TwitterURL, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying builder by handle: %w", err)
	}
	return b, nil
}

func (s *BuilderStore) GetByID(id int64) (*model.Builder, error) {
	b := &model.Builder{}
	err := s.db.QueryRow(
		`SELECT id, handle, display_name, bio, avatar_url, website, github_url, twitter_url, created_at, updated_at
		 FROM builders WHERE id = ?`, id,
	).Scan(&b.ID, &b.Handle, &b.DisplayName, &b.Bio, &b.AvatarURL, &b.Website, &b.GithubURL, &b.TwitterURL, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying builder by id: %w", err)
	}
	return b, nil
}

func (s *BuilderStore) Create(req model.CreateBuilderRequest) (*model.Builder, error) {
	// Validate invitation code
	var codeUsedBy sql.NullInt64
	err := s.db.QueryRow(`SELECT used_by FROM invitation_codes WHERE code = ?`, req.InvitationCode).Scan(&codeUsedBy)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid invitation code")
	}
	if err != nil {
		return nil, fmt.Errorf("checking invitation code: %w", err)
	}
	if codeUsedBy.Valid {
		return nil, fmt.Errorf("invitation code already used")
	}

	now := time.Now().UTC()
	result, err := s.db.Exec(
		`INSERT INTO builders (handle, display_name, bio, avatar_url, website, github_url, twitter_url, invitation_code, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Handle, req.DisplayName, req.Bio, req.AvatarURL, req.Website, req.GithubURL, req.TwitterURL, req.InvitationCode, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting builder: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}

	// Mark invitation code as used
	_, err = s.db.Exec(`UPDATE invitation_codes SET used_by = ?, used_at = ? WHERE code = ?`, id, now, req.InvitationCode)
	if err != nil {
		return nil, fmt.Errorf("marking invitation code used: %w", err)
	}

	return s.GetByID(id)
}

func (s *BuilderStore) UpdateAvatar(id int64, avatarURL string) (*model.Builder, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE builders SET avatar_url = ?, updated_at = ? WHERE id = ?`,
		avatarURL, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating builder avatar: %w", err)
	}
	return s.GetByID(id)
}

func (s *BuilderStore) Update(id int64, displayName, bio, website, twitterURL string) (*model.Builder, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE builders SET display_name = ?, bio = ?, website = ?, twitter_url = ?, updated_at = ? WHERE id = ?`,
		displayName, bio, website, twitterURL, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating builder: %w", err)
	}
	return s.GetByID(id)
}
